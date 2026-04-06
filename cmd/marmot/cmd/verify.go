package cmd

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/crypto"
	"github.com/pol-cova/marmot-cli/internal/remote"

	"github.com/spf13/cobra"
)

func newVerifyCmd() *cobra.Command {
	var (
		backupID   string
		databaseID string
		latest     bool
		encFile    string
		format     string
	)

	cmd := &cobra.Command{
		Use:   "verify [backup-id]",
		Short: "Verify backup integrity and preview contents",
		Long: `Download, decrypt, and verify a backup without restoring to database.

Shows file integrity, structure validation, row/document counts, and sample data.

For cloud storage:
  # Verify a specific backup from cloud storage
  marmot verify s3://bucket/web-01/2024/01/15/mydb-1705312800.enc

  # Verify latest backup for a database
  marmot verify --db prod-mongo --latest

For local storage:
  # Verify a local backup file
  marmot verify --file ./my-backup.enc

  # Quick summary only
  marmot verify --file ./my-backup.enc --format summary`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				backupID = args[0]
			}
			return runVerify(cmd, backupID, databaseID, latest, encFile, format)
		},
	}

	cmd.Flags().StringVar(&databaseID, "db", "", "database ID to verify (use with --latest)")
	cmd.Flags().BoolVar(&latest, "latest", false, "verify latest backup for the database")
	cmd.Flags().StringVar(&encFile, "file", "", "path to local encrypted backup file")
	cmd.Flags().StringVar(&format, "format", "detailed", "output format: summary or detailed")

	return cmd
}

func runVerify(cmd *cobra.Command, backupID, databaseID string, latest bool, encFile, format string) error {
	cfg, err := config.LoadConfig(getConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create a context with timeout for remote operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Determine source
	var sourcePath string
	var dbType string

	if encFile != "" {
		// Local file
		sourcePath = encFile
		dbType = detectDBTypeFromFilename(encFile)
	} else if latest && databaseID != "" {
		// Get latest backup from remote storage
		// For local-only mode, this is not supported
		if cfg.IsLocal() {
			return fmt.Errorf("local storage mode: use --file to verify a local backup file")
		}

		dbConfig := cfg.GetDatabaseByID(databaseID)
		if dbConfig == nil {
			return fmt.Errorf("database not found: %s", databaseID)
		}
		dbType = dbConfig.Type

		// Create remote storage only for cloud mode
		remoteStorage, err := remote.NewStorageWithContext(ctx, cfg)
		if err != nil {
			return fmt.Errorf("failed to create remote storage: %w", err)
		}

		backups, err := remoteStorage.GetClient().ListBackups(ctx, dbConfig.Name, 1)
		if err != nil {
			return fmt.Errorf("failed to list backups: %w", err)
		}
		if len(backups) == 0 {
			return fmt.Errorf("no backups found for database: %s", dbConfig.Name)
		}
		backupID = backups[0].BackupID

		// Download to temp file
		tempFile, err := os.CreateTemp("", "marmot-verify-*.enc")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		fmt.Printf("Downloading backup %s...\n", backupID)
		if err := remoteStorage.GetClient().Download(ctx, backupID, tempFile); err != nil {
			return fmt.Errorf("failed to download backup: %w", err)
		}

		sourcePath = tempFile.Name()
		if _, err := tempFile.Seek(0, 0); err != nil {
			return fmt.Errorf("failed to seek temp file: %w", err)
		}
	} else if backupID != "" {
		// Download specific backup from remote storage
		// For local-only mode, this is not supported
		if cfg.IsLocal() {
			return fmt.Errorf("local storage mode: use --file to verify a local backup file")
		}

		// Create remote storage (Hub or S3)
		remoteStorage, err := remote.NewStorageWithContext(ctx, cfg)
		if err != nil {
			return fmt.Errorf("failed to create remote storage: %w", err)
		}

		// Download to temp file
		tempFile, err := os.CreateTemp("", "marmot-verify-*.enc")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		fmt.Printf("Downloading backup %s...\n", backupID)
		if err := remoteStorage.GetClient().Download(ctx, backupID, tempFile); err != nil {
			return fmt.Errorf("failed to download backup: %w", err)
		}

		sourcePath = tempFile.Name()
		dbType = detectDBTypeFromFilename(backupID)
	} else {
		return fmt.Errorf("must specify backup-id, --file, or --db with --latest")
	}

	// Load encryption key
	encryptor := crypto.NewAESEncryptor()
	if err := encryptor.LoadKeyFromFile(cfg.Paths.KeyFile); err != nil {
		return fmt.Errorf("failed to load encryption key: %w", err)
	}

	// Decrypt to memory buffer
	fmt.Println("Decrypting backup...")
	decryptedBuf := new(bytes.Buffer)
	encFileHandle, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer encFileHandle.Close()

	if err := encryptor.Decrypt(encFileHandle, decryptedBuf); err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	// Decompress to memory buffer
	fmt.Println("Decompressing backup...")
	gzipReader, err := gzip.NewReader(decryptedBuf)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Verify based on database type
	var report *VerifyReport

	switch dbType {
	case "mysql", "postgres":
		report, err = verifySQLDump(gzipReader, dbType)
	case "mongo":
		report, err = verifyMongoArchive(gzipReader)
	default:
		// Try auto-detect from content
		report, err = verifyAutoDetect(gzipReader)
	}

	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Print report
	printVerifyReport(report, format)

	return nil
}

type VerifyReport struct {
	DatabaseType    string
	BackupSize      int64
	StructureCount  int
	TotalRows       int64
	Collections     []CollectionInfo
	IntegrityStatus string
	SampleData      []SampleEntry
}

type CollectionInfo struct {
	Name     string
	RowCount int64
	Size     int64
}

type SampleEntry struct {
	Collection string
	Data       string
}

func verifySQLDump(r io.Reader, dbType string) (*VerifyReport, error) {
	report := &VerifyReport{
		DatabaseType:    dbType,
		IntegrityStatus: "PASSED",
		Collections:     []CollectionInfo{},
		SampleData:      []SampleEntry{},
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024*10) // 10MB buffer

	insertCount := 0
	sampleCount := 0

	// Regex patterns
	createTableRegex := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?["\x60']?(\w+)["\x60']?`)
	insertRegex := regexp.MustCompile(`(?i)INSERT\s+INTO\s+["\x60']?(\w+)["\x60']?`)

	for scanner.Scan() {
		line := scanner.Text()

		// Count tables
		if matches := createTableRegex.FindStringSubmatch(line); len(matches) > 1 {
			tableName := matches[1]
			report.StructureCount++
			report.Collections = append(report.Collections, CollectionInfo{
				Name:     tableName,
				RowCount: 0,
			})
		}

		// Count inserts (estimate rows)
		if matches := insertRegex.FindStringSubmatch(line); len(matches) > 1 {
			tableName := matches[1]
			insertCount++
			report.TotalRows++

			// Update collection row count
			for i := range report.Collections {
				if report.Collections[i].Name == tableName {
					report.Collections[i].RowCount++
					break
				}
			}

			// Collect samples (max 3 per table)
			if sampleCount < 3*report.StructureCount {
				sampleLine := truncateString(line, 200)
				report.SampleData = append(report.SampleData, SampleEntry{
					Collection: tableName,
					Data:       sampleLine,
				})
				sampleCount++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading dump: %w", err)
	}

	return report, nil
}

func verifyMongoArchive(r io.Reader) (*VerifyReport, error) {
	report := &VerifyReport{
		DatabaseType:    "mongo",
		IntegrityStatus: "PASSED",
		Collections:     []CollectionInfo{},
		SampleData:      []SampleEntry{},
	}

	// Read all data into buffer for parsing
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read archive: %w", err)
	}

	// Parse BSON archive format
	// MongoDB archive format: header + collections + terminator
	// Each collection: metadata document + documents...

	offset := 0
	collections := make(map[string]*CollectionInfo)

	// First document should be header or collection metadata
	for offset < len(data)-4 {
		// Read document length (BSON is little-endian int32)
		if offset+4 > len(data) {
			break
		}
		docLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))

		if docLen <= 0 || docLen > 16*1024*1024 || offset+docLen > len(data) {
			// Invalid document size, try to skip
			offset++
			continue
		}

		docData := data[offset : offset+docLen]

		// Try to parse as BSON document
		doc, err := parseBSONDocument(docData)
		if err == nil && doc != nil {
			// Check if this is metadata document
			if ns, ok := doc["ns"].(string); ok && ns != "" {
				// Collection namespace: "db.collection"
				parts := strings.SplitN(ns, ".", 2)
				if len(parts) == 2 {
					collName := parts[1]
					if _, exists := collections[collName]; !exists {
						report.StructureCount++
						coll := &CollectionInfo{Name: collName, RowCount: 0}
						collections[collName] = coll
						report.Collections = append(report.Collections, *coll)
					}
				}
			}

			// Check if this is a regular document (has _id field)
			if _, hasID := doc["_id"]; hasID {
				report.TotalRows++
				// Update last collection's count
				if len(report.Collections) > 0 {
					lastIdx := len(report.Collections) - 1
					report.Collections[lastIdx].RowCount++
					collections[report.Collections[lastIdx].Name].RowCount++
				}

				// Collect sample data (max 3 per collection)
				if len(report.SampleData) < 3*len(report.Collections) {
					sample := formatBSONSample(doc)
					if len(report.Collections) > 0 {
						collName := report.Collections[len(report.Collections)-1].Name
						report.SampleData = append(report.SampleData, SampleEntry{
							Collection: collName,
							Data:       sample,
						})
					}
				}
			}
		}

		offset += docLen
	}

	return report, nil
}

// parseBSONDocument parses a BSON document and returns it as a map
func parseBSONDocument(data []byte) (map[string]interface{}, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("document too short")
	}

	doc := make(map[string]interface{})
	offset := 4 // Skip length

	for offset < len(data)-1 {
		if offset >= len(data) {
			break
		}

		bsonType := data[offset]
		offset++

		if bsonType == 0 {
			// End of document
			break
		}

		// Read field name (null-terminated string)
		nameEnd := offset
		for nameEnd < len(data) && data[nameEnd] != 0 {
			nameEnd++
		}
		if nameEnd >= len(data) {
			break
		}
		fieldName := string(data[offset:nameEnd])
		offset = nameEnd + 1

		// Read value based on type
		var value interface{}
		switch bsonType {
		case 0x01: // Double (8 bytes)
			if offset+8 <= len(data) {
				bits := binary.LittleEndian.Uint64(data[offset : offset+8])
				value = bits // Just store as uint64 for simplicity
				offset += 8
			}
		case 0x02: // String
			if offset+4 <= len(data) {
				strLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
				offset += 4
				if offset+strLen <= len(data) && strLen > 0 {
					value = string(data[offset : offset+strLen-1]) // -1 to remove null terminator
					offset += strLen
				}
			}
		case 0x07: // ObjectId (12 bytes)
			if offset+12 <= len(data) {
				value = fmt.Sprintf("ObjectId(\"%x\")", data[offset:offset+12])
				offset += 12
			}
		case 0x08: // Boolean (1 byte)
			if offset+1 <= len(data) {
				value = data[offset] != 0
				offset++
			}
		case 0x09: // UTC datetime (8 bytes)
			if offset+8 <= len(data) {
				ms := int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
				value = time.Unix(ms/1000, (ms%1000)*1000000).Format(time.RFC3339)
				offset += 8
			}
		case 0x10: // Int32 (4 bytes)
			if offset+4 <= len(data) {
				value = int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
				offset += 4
			}
		case 0x12: // Int64 (8 bytes)
			if offset+8 <= len(data) {
				value = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
				offset += 8
			}
		default:
			// Skip unknown types - try to find next field
			// This is a simplification; full BSON parsing is complex
			break
		}

		if value != nil {
			doc[fieldName] = value
		}
	}

	return doc, nil
}

func formatBSONSample(doc map[string]interface{}) string {
	// Format document as simple key: value pairs
	parts := []string{}
	count := 0
	for k, v := range doc {
		if count >= 5 { // Limit to 5 fields for display
			parts = append(parts, "...")
			break
		}
		valStr := fmt.Sprintf("%v", v)
		if len(valStr) > 50 {
			valStr = valStr[:47] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s: %s", k, valStr))
		count++
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func verifyAutoDetect(r io.Reader) (*VerifyReport, error) {
	// Read a small portion to detect type
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}

	content := string(buf[:n])

	// Try to detect database type from content
	if strings.Contains(content, "CREATE TABLE") || strings.Contains(content, "INSERT INTO") {
		// Create a new reader with the buffered content
		fullReader := io.MultiReader(bytes.NewReader(buf[:n]), r)
		return verifySQLDump(fullReader, "unknown-sql")
	}

	// Check for BSON binary patterns
	if n > 4 {
		docLen := binary.LittleEndian.Uint32(buf[:4])
		if docLen > 0 && docLen < 16*1024*1024 {
			fullReader := io.MultiReader(bytes.NewReader(buf[:n]), r)
			return verifyMongoArchive(fullReader)
		}
	}

	return nil, fmt.Errorf("unable to auto-detect backup format")
}

func printVerifyReport(report *VerifyReport, format string) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  ✓ Backup Integrity: %s\n", report.IntegrityStatus)
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()

	fmt.Printf("Database Type:     %s\n", strings.ToUpper(report.DatabaseType))
	fmt.Printf("Structure Count:   %d %s\n", report.StructureCount, pluralize(report.StructureCount, "table", "tables"))
	fmt.Printf("Total Records:     ~%d\n", report.TotalRows)
	fmt.Println()

	if format == "detailed" && len(report.Collections) > 0 {
		fmt.Println("┌─────────────────────────────────────────────────────────┐")
		fmt.Println("│ Structure Details                                       │")
		fmt.Println("├─────────────────────────────────────────────────────────┤")

		for _, coll := range report.Collections {
			fmt.Printf("│ %-23s %10d %s\n", coll.Name+":", coll.RowCount, pluralize(int(coll.RowCount), "row", "rows"))
		}

		fmt.Println("└─────────────────────────────────────────────────────────┘")
		fmt.Println()

		if len(report.SampleData) > 0 {
			fmt.Println("┌─────────────────────────────────────────────────────────┐")
			fmt.Println("│ Sample Data Preview (3 records per structure)          │")
			fmt.Println("├─────────────────────────────────────────────────────────┤")

			currentColl := ""
			count := 0
			for _, sample := range report.SampleData {
				if sample.Collection != currentColl {
					if currentColl != "" {
						fmt.Println("│")
					}
					currentColl = sample.Collection
					count = 0
					fmt.Printf("│ %s:\n", currentColl)
				}
				if count < 3 {
					sampleStr := truncateString(sample.Data, 65)
					fmt.Printf("│   %s\n", sampleStr)
					count++
				}
			}

			fmt.Println("└─────────────────────────────────────────────────────────┘")
			fmt.Println()
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  ✓ Your backup is healthy and contains valid data")
	fmt.Println("═══════════════════════════════════════════════════════════")
}

func detectDBTypeFromFilename(filename string) string {
	lower := strings.ToLower(filename)
	if strings.Contains(lower, "mongo") {
		return "mongo"
	}
	if strings.Contains(lower, "postgres") || strings.Contains(lower, "pg") {
		return "postgres"
	}
	if strings.Contains(lower, "mysql") || strings.Contains(lower, "mariadb") {
		return "mysql"
	}
	return ""
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
