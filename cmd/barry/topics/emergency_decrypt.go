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
		err := emergencyDecrypt(args)
		if err != nil {
			log.Fatal(err.Error())
		}
	},
}

func emergencyDecrypt(args []string) error {
	infile, err := os.Open(args[0])
	if err != nil {
		log.Fatal(err.Error())
	}
	defer infile.Close()

	outfile, err := os.Create(args[1])
	if err != nil {
		return err
	}

	success := false
	defer func() {
		outfile.Close()
		if !success {
			fmt.Printf("Error: file '%s' NOT decrypted\n", args[0])
			os.Remove(args[1])
		}
	}()

	key, err := base64.StdEncoding.DecodeString(args[2])
	if err != nil {
		return err
	}

	err = common.DecryptFile(infile, outfile, func(keyName string) ([]byte, error) {
		fmt.Printf("Info: file was encrypted with key '%s'\n", keyName)
		return key, nil
	})

	if err != nil {
		return err
	}

	fmt.Printf("File '%s' decrypted to '%s'\n", args[0], args[1])
	success = true
	return nil
}

func init() {
	emergencyCmd.AddCommand(emergencyDecryptCmd)
}
