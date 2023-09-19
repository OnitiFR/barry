package topics

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/OnitiFR/barry/common"
	"github.com/spf13/cobra"
)

// emergencyDecryptCmd represents the file command
var emergencyDecryptCmd = &cobra.Command{
	Use:   "decrypt <encrypted-file> <output-file> <base64-key>",
	Short: "Decrypt a file locally using a project key",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		infile, err := os.Open(args[0])
		if err != nil {
			log.Fatal(err.Error())
		}
		defer infile.Close()

		outfile, err := os.Create(args[1])
		if err != nil {
			log.Fatal(err.Error())
		}
		defer outfile.Close()

		key, err := base64.StdEncoding.DecodeString(args[2])
		if err != nil {
			log.Fatal(err.Error())
		}

		err = common.DecryptFile(infile, outfile, func(keyName string) ([]byte, error) {
			fmt.Printf("Info: file was encrypted with key '%s'\n", keyName)
			return key, nil
		})

		if err != nil {
			log.Fatal(err.Error())
		}

		fmt.Printf("File '%s' decrypted to '%s'\n", args[0], args[1])
	},
}

func init() {
	emergencyCmd.AddCommand(emergencyDecryptCmd)
}
