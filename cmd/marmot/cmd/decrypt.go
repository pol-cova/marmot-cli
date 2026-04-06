package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pol-cova/marmot-cli/internal/backup"
	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/crypto"

	"github.com/spf13/cobra"
)

func newDecryptCmd() *cobra.Command {
	var inFile string
	var outFile string

	cmd := &cobra.Command{
		Use:   "decrypt",
		Short: "Decrypt and decompress a local .enc backup file without restoring",
		Long:  "Decrypts a Marmot-encrypted backup (.enc) using your key and decompresses it to a plain dump file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDecrypt(cmd, inFile, outFile)
		},
	}

	cmd.Flags().StringVar(&inFile, "file", "", "path to the local encrypted .enc file (required)")
	cmd.Flags().StringVar(&outFile, "out", "", "output file path (default: input file without .enc)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func runDecrypt(cmd *cobra.Command, inFile, outFile string) error {
	cfg, err := config.LoadConfig(getConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Prepare encryptor (needs the key file)
	enc := crypto.NewAESEncryptor()
	if err := enc.LoadKeyFromFile(cfg.Paths.KeyFile); err != nil {
		return fmt.Errorf("failed to load encryption key: %w", err)
	}

	// Determine output file path
	if outFile == "" {
		base := inFile
		if strings.HasSuffix(strings.ToLower(base), ".enc") {
			base = strings.TrimSuffix(base, filepath.Ext(base))
		}
		outFile = base
	}

	// Open input .enc file
	in, err := os.Open(inFile)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer in.Close()

	// Temp file for decrypted gzip content
	decryptedGz, err := os.CreateTemp("", "marmot-decrypted-*.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		name := decryptedGz.Name()
		decryptedGz.Close()
		os.Remove(name)
	}()

	// Decrypt to gz
	if err := enc.Decrypt(in, decryptedGz); err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}
	if _, err := decryptedGz.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek decrypted file: %w", err)
	}

	// Create output file
	out, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	// Decompress gz -> out
	comp := backup.NewCompressor()
	if err := comp.Decompress(decryptedGz, out); err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}

	fmt.Printf("Decrypted and extracted to: %s\n", outFile)
	return nil
}
