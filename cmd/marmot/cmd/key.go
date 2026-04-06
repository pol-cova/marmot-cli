package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/crypto"
	"github.com/spf13/cobra"
)

func newKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage encryption key",
		Long:  "Export or import the encryption key for backup portability and disaster recovery",
	}
	cmd.AddCommand(newKeyExportCmd())
	cmd.AddCommand(newKeyImportCmd())
	return cmd
}

func newKeyExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export encryption key as base64 to stdout",
		Long: `Prints the encryption key as a base64 string.

IMPORTANT: Store this output somewhere safe OUTSIDE this server (password manager,
printed paper, secure offline storage). Without this key, encrypted backups CANNOT
be decrypted if this server is lost or wiped by ransomware.

Example:
  marmot key export > marmot.key      # save to file, then move off server
  marmot key export | pbcopy          # copy to clipboard (macOS)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := config.GetDefaultPaths()
			enc := crypto.NewAESEncryptor()
			if err := enc.LoadKeyFromFile(paths.KeyFile); err != nil {
				return fmt.Errorf("failed to load key from %s: %w", paths.KeyFile, err)
			}
			b64, err := enc.ExportKeyAsBase64()
			if err != nil {
				return fmt.Errorf("failed to export key: %w", err)
			}
			fmt.Println(b64)
			fmt.Fprintln(os.Stderr, "\n[!] Store this key safely OUTSIDE this server.")
			fmt.Fprintln(os.Stderr, "    Example: marmot key export > marmot.key")
			fmt.Fprintln(os.Stderr, "    To restore on a new server: marmot key import <base64>")
			return nil
		},
	}
}

func newKeyImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <base64-key>",
		Short: "Import an encryption key from base64",
		Long: `Imports a previously exported key and saves it to key.bin.

Use this to restore decryption capability on a new or recovered server after the
original was lost to ransomware, hardware failure, or other disasters.

Example:
  marmot key import "base64encodedkeyhere=="`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b64 := strings.TrimSpace(args[0])
			keyBytes, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				return fmt.Errorf("invalid base64 key: %w", err)
			}

			paths := config.GetDefaultPaths()
			if err := os.MkdirAll(paths.ConfigDir, 0700); err != nil {
				return fmt.Errorf("failed to create config dir: %w", err)
			}

			enc := crypto.NewAESEncryptor()
			if err := enc.LoadKey(keyBytes); err != nil {
				return fmt.Errorf("invalid key (must be 32 bytes / AES-256): %w", err)
			}
			if err := enc.SaveKeyToFile(paths.KeyFile); err != nil {
				return fmt.Errorf("failed to save key to %s: %w", paths.KeyFile, err)
			}

			fmt.Printf("Key imported and saved to: %s\n", paths.KeyFile)
			return nil
		},
	}
}
