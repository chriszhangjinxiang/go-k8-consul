package cmd

import
(
	"demo/imp"
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var k8path string
var consuladdress string

var k8Cmd = &cobra.Command{
	Use:   "fist",
	Short: "fist is a tool to monitor kubernetes service",
	Long: `this tool is to monitor k8 services & pod and do the resistration to consul
you need to provide to paramters
k8path -- k8 config file path, normally you can find the config file in ~/.kube location
consuladdress  -- consul ip address & port `,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
	Run: func(cmd *cobra.Command, args []string) {
		if len(consuladdress) == 0 || len(consuladdress) == 0 {
			cmd.Help()
			return
		}
		//imp.Show2(service, deployment)
		imp.GetService(k8path,consuladdress)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := k8Cmd.Execute(); err != nil {
		fmt.Println(err)
		//os.Exit(1)
		os.Exit(-1)
	}
}

func init() {
	k8Cmd.Flags().StringVarP(&k8path, "k8path", "k", "", "K8 config file path")
	k8Cmd.Flags().StringVarP(&consuladdress, "consuladdress", "c", "", "consul IP address:port")
}