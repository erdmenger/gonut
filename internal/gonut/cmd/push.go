// Copyright © 2019 The Homeport Team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/homeport/gonut/internal/gonut/assets"
	"github.com/homeport/gonut/internal/gonut/cf"
	"github.com/homeport/gonvenience/pkg/v1/bunt"
	"github.com/homeport/gonvenience/pkg/v1/text"
	"github.com/homeport/pina-golada/pkg/files"
)

//GonutAppPrefix is the prefeix for gonuts applications, it is also used by
//the cleanup command to decide whether an app is pushed by gonut or not
var GonutAppPrefix = "gonut"

type sampleApp struct {
	caption       string
	command       string
	aliases       []string
	appNamePrefix string
	assetFunc     func() (files.Directory, error)
}

var (
	deleteSetting  string
	summarySetting string
	outputSetting  string
)

var sampleApps = []sampleApp{
	{
		caption:       "Golang",
		command:       "golang",
		aliases:       []string{"go"},
		appNamePrefix: fmt.Sprintf("%s-golang-app-", GonutAppPrefix),
		assetFunc:     assets.Provider.GoSampleApp,
	},

	{
		caption:       "Python",
		command:       "python",
		aliases:       []string{},
		appNamePrefix: fmt.Sprintf("%s-python-app-", GonutAppPrefix),
		assetFunc:     assets.Provider.PythonSampleApp,
	},

	{
		caption:       "PHP",
		command:       "php",
		aliases:       []string{},
		appNamePrefix: fmt.Sprintf("%s-php-app-", GonutAppPrefix),
		assetFunc:     assets.Provider.PHPSampleApp,
	},

	{
		caption:       "Staticfile",
		command:       "staticfile",
		aliases:       []string{"static"},
		appNamePrefix: fmt.Sprintf("%s-staticfile-app-", GonutAppPrefix),
		assetFunc:     assets.Provider.StaticfileSampleApp,
	},

	{
		caption:       "Swift",
		command:       "swift",
		aliases:       []string{},
		appNamePrefix: fmt.Sprintf("%s-swift-app-", GonutAppPrefix),
		assetFunc:     assets.Provider.SwiftSampleApp,
	},

	{
		caption:       "NodeJS",
		command:       "nodejs",
		aliases:       []string{"node"},
		appNamePrefix: fmt.Sprintf("%s-nodejs-app-", GonutAppPrefix),
		assetFunc:     assets.Provider.NodeJSSampleApp,
	},

	{
		caption:       "Ruby",
		command:       "ruby",
		appNamePrefix: fmt.Sprintf("%s-ruby-sinatra-app-", GonutAppPrefix),
		assetFunc:     assets.Provider.RubySampleApp,
	},
}

// pushCmd represents the push command
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push a sample app to Cloud Foundry",
	Long:  `Use one of the sub-commands to select a sample app of a list of programming languages to be pushed to a Cloud Foundry instance.`,
}

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.PersistentFlags().StringVarP(&deleteSetting, "delete", "d", "always", "Delete application after push: always, never, on-success")
	pushCmd.PersistentFlags().StringVarP(&summarySetting, "summary", "s", "short", "Push summary detail level: quiet, short, full")
	pushCmd.PersistentFlags().StringVarP(&outputSetting, "output", "o", "", "Push summary type: json | yaml")

	for _, sampleApp := range sampleApps {
		pushCmd.AddCommand(&cobra.Command{
			Use:     sampleApp.command,
			Aliases: sampleApp.aliases,
			Short:   fmt.Sprintf("Push a %s sample app to Cloud Foundry", sampleApp.caption),
			Long:    fmt.Sprintf(`Push a %s sample app to Cloud Foundry. The application will be deleted after it was pushed successfully.`, sampleApp.caption),
			Run:     genericCommandFunc,
		})
	}

	pushCmd.AddCommand(&cobra.Command{
		Use:   "all",
		Short: "Pushes all available sample apps to Cloud Foundry",
		Long:  `Pushes all available sample apps to Cloud Foundry. Each application will be deleted after it was pushed successfully.`,
		Run: func(cmd *cobra.Command, args []string) {
			for _, sampleApp := range sampleApps {
				if err := runSampleAppPush(sampleApp); err != nil {
					ExitGonut(err)
				}
			}
		},
	})
}

func lookUpSampleAppByName(name string) *sampleApp {
	for _, sampleApp := range sampleApps {
		if sampleApp.command == name {
			return &sampleApp
		}
	}

	return nil
}

func genericCommandFunc(cmd *cobra.Command, args []string) {
	sampleApp := lookUpSampleAppByName(cmd.Use)
	if sampleApp == nil {
		ExitGonut("failed to detect which sample app is to be tested")
	}

	if err := runSampleAppPush(*sampleApp); err != nil {
		ExitGonut(err)
	}
}

func mapDeleteSetting(deleteSetting string) (cf.AppCleanupSetting, error) {

	switch deleteSetting {
	case "always":
		return cf.Always, nil
	case "never":
		return cf.Never, nil

	case "on-success":
		return cf.OnSuccess, nil

	default:
		return -1, fmt.Errorf("unsupported delete setting: %s", deleteSetting)
	}
}

func mapOutputSetting(outputSetting string) (cf.OutputType, error) {

	switch strings.ToLower(outputSetting) {
	case "json":
		return cf.JSON, nil
	case "yaml":
		return cf.YAML, nil
	default:
		// return -1, fmt.Errorf("unsupported output type: %s", outputSetting)
		return -1, nil
	}
}

func summaryPrintout(appCaption string, report *cf.PushReport, summarySetting string, outputType cf.OutputType) error {

	switch summarySetting {
	case "short", "oneline":
		bunt.Printf("Successfully pushed *%s* sample app in CadetBlue{%s}.\n",
			appCaption,
			humanReadableDuration(report.ElapsedTime()),
		)

	case "full":
		switch outputType {
		case cf.JSON:

			buffer := bytes.Buffer{}

			output, err := report.ToJSON()
			if err != nil {
				return err
			}

			indentError := json.Indent(&buffer, output, "", "   ")
			if indentError != nil {
				return indentError
			}

			bunt.Println(buffer.String())

		case cf.YAML:
			output, err := report.ToYAML()
			if err != nil {
				return err
			}

			bunt.Println(string(output))

		default:
			bunt.Printf("Successfully pushed *%s* sample app in CadetBlue{%s}:\n", appCaption, humanReadableDuration(report.ElapsedTime()))
			bunt.Printf("     DimGray{_stack:_} DarkSeaGreen{%s}\n", report.Stack())
			bunt.Printf(" DimGray{_buildpack:_} DarkSeaGreen{%s}\n", report.Buildpack())
			if report.HasTimeDetails() {
				bunt.Printf("   DimGray{_ramp-up:_} SteelBlue{%s}\n", humanReadableDuration(report.InitTime()))
				bunt.Printf("  DimGray{_creating:_} SteelBlue{%s}\n", humanReadableDuration(report.CreatingTime()))
				bunt.Printf(" DimGray{_uploading:_} SteelBlue{%s}\n", humanReadableDuration(report.UploadingTime()))
				bunt.Printf("   DimGray{_staging:_} SteelBlue{%s}\n", humanReadableDuration(report.StagingTime()))
				bunt.Printf("  DimGray{_starting:_} SteelBlue{%s}\n", humanReadableDuration(report.StartingTime()))
			}
			bunt.Printf("\n")
		}

	}

	return nil
}

func runSampleAppPush(app sampleApp) error {

	cleanupSetting, err := mapDeleteSetting(deleteSetting)
	if err != nil {
		return err
	}

	outputType, err := mapOutputSetting(outputSetting)
	if err != nil {
		return err
	}

	appName := text.RandomStringWithPrefix(app.appNamePrefix, 32)

	directory, err := app.assetFunc()
	if err != nil {
		return err
	}

	report, err := cf.PushApp(app.caption, appName, directory, cleanupSetting)
	if err != nil {
		return err
	}

	return summaryPrintout(app.caption, report, summarySetting, outputType)
}

func humanReadableDuration(duration time.Duration) string {
	if duration < time.Second {
		return "less than a second"
	}

	seconds := int(duration.Seconds())
	minutes := 0
	hours := 0

	if seconds >= 60 {
		minutes = seconds / 60
		seconds = seconds % 60

		if minutes >= 60 {
			hours = minutes / 60
			minutes = minutes % 60
		}
	}

	parts := []string{}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d h", hours))
	}

	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%d min", minutes))
	}

	if seconds > 0 {
		parts = append(parts, fmt.Sprintf("%d sec", seconds))
	}

	return strings.Join(parts, " ")
}
