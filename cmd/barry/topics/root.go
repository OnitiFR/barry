package topics

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/OnitiFR/barry/cmd/barry/client"
	"github.com/OnitiFR/barry/common"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "barry",
	Short: "Barry CLI client",
	Long: `Barry will send your backups into the clouds

Sample usage:
- barry project list
- barry project list <project>
- barry file download <project> <file>
	`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s\n", cmd.Short)
		fmt.Printf("%s\n", cmd.Long)
		fmt.Printf("Use --help to list commands and options.\n\n")
		fmt.Printf("configuration file '%s'\n",
			client.GlobalConfig.ConfigFile,
		)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	var err error
	client.GlobalHome, err = homedir.Dir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}

	if err = rootCmd.Execute(); err != nil {
		return err
	}
	return nil
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&client.GlobalCfgFile, "config", "c", "", "config file (default is $HOME/.barry.toml)")

	rootCmd.PersistentFlags().BoolP("trace", "t", false, "also show server TRACE messages (debug)")
	rootCmd.PersistentFlags().BoolP("time", "d", false, "show server timestamps on messages")
	rootCmd.PersistentFlags().BoolP("version", "v", false, "show client version")

	rootCmd.PersistentFlags().BoolP("get-config-filename", "", false, "get current config filename (useful for completion)")
	rootCmd.PersistentFlags().MarkHidden("get-config-filename")
}

func setCompletion() {
	rootCmd.BashCompletionFunction = bashCompletionFunc
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {

	cfgFile := client.GlobalCfgFile
	if cfgFile == "" {
		cfgFile = path.Clean(client.GlobalHome + "/.barry.toml")
	}

	var err error
	client.GlobalConfig, err = NewRootConfig(cfgFile)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}

	if client.GlobalConfig == nil {
		fmt.Printf(`ERROR: Configuration file not found: %s

Example:

url = "http://192.168.10.104:8787"
key = "gein2xah7keeL33thpe9ahvaegF15TUL3surae3Chue4riokooJ5WuTI80FTWfz2"

other settings: trace, time (boolean)
Note: you can also use environment variables (TRACE, TIME).
`, cfgFile)
		os.Exit(1)
	}

	client.GlobalAPI = client.NewAPI(
		client.GlobalConfig.URL,
		client.GlobalConfig.Key,
		client.GlobalConfig.Trace,
		client.GlobalConfig.Time,
	)

	setCompletion()

	if rootCmd.PersistentFlags().Lookup("get-config-filename").Changed {
		fmt.Println(cfgFile)
		os.Exit(0)
	}

	if rootCmd.PersistentFlags().Lookup("version").Changed {
		fmt.Println(common.ClientVersion)
		os.Exit(0)
	}
}
