package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/casimir/xdg-go"
	"github.com/codegangsta/cli"
	"github.com/codegangsta/cli/altsrc"
	"github.com/jroimartin/gocui"
)

// Version is the application version (follows http://semver.org/)
const Version = "0.0.1"

// Config is an application configuration object
type Config struct {
	ConfigFile      string      `config-short:"config" config-name:"Configuration file"`
	Email           string      `config-short:"email" config-name:"Email address"`
	APIKey          string      `config-short:"apikey" config-name:"API key"`
	APIBase         string      `config-short:"site" config-name:"API base URL"`
	Secure          bool        `config-short:"secure" config-name:"Secure connection"`
	PromptColor     string      `config-short:"prompt-color" config-name:"Prompt color"`
	Prompt          string      `config-short:"prompt" config-name:"Prompt"`
	RLHistory       bool        `config-short:"history" config-name:"History"`
	RLHistoryFile   string      `config-short:"history-file" config-name:"History file"`
	CacheFile       string      `config-short:"cache-file" config-name:"Cache file"`
	Logging         bool        `config-short:"logging" config-name:"Logging"`
	LogFile         string      `config-short:"log-file" config-name:"Log file"`
	XDGApp          xdg.App     `config-short:"-" config-name:"-"`
	CLIApp          cli.App     `config-short:"-" config-name:"-"`
	Interface       *gocui.Gui  `config-short:"-" config-name:"-"`
	MainTextChannel chan string `config-short:"-" config-name:"-"`
}

// Handles command line arguments and help printing
func commandLineSetup() cli.App {
	cliApp := cli.NewApp()
	cliApp.Name = "clisiana"
	cliApp.Usage = "A command line Zulip client"
	cliApp.Version = Version
	cliApp.Authors = []cli.Author{cli.Author{Name: "Michael Cordover", Email: "public@mjec.net"}}
	cliApp.Copyright = "This program is distributed under the GNU General Public License version 3."
	cli.AppHelpTemplate = `{{.Name}} - {{.Usage}}
Usage: {{.HelpName}} {{if .VisibleFlags}}[options]{{end}}
{{range .VisibleFlags}}
   {{.}}{{end}}
`
	cliApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "config,c",
			Value:       config.XDGApp.ConfigPath("clisiana.yaml"),
			Usage:       "The YAML file to load configuration from",
			Destination: &config.ConfigFile,
			EnvVar:      "CLISIANA_CONFIG",
		},
		altsrc.NewStringFlag(cli.StringFlag{
			Name:        "email",
			Value:       "",
			Usage:       "Your Zulip email address",
			Destination: &config.Email,
			EnvVar:      "CLISIANA_ZULIP_EMAIL,ZULIP_EMAIL",
		}),
		altsrc.NewStringFlag(cli.StringFlag{
			Name:        "apikey",
			Value:       "",
			Usage:       "Your Zulip API Key",
			Destination: &config.APIKey,
			EnvVar:      "CLISIANA_ZULIP_API_KEY,ZULIP_API_KEY",
		}),
		altsrc.NewStringFlag(cli.StringFlag{
			Name:        "site",
			Value:       "https://api.zulip.com/v1/",
			Usage:       "The base URL of the Zulip API to connect to",
			Destination: &config.APIBase,
			EnvVar:      "CLISIANA_ZULIP_URL,ZULIP_URL",
		}),
		altsrc.NewBoolTFlag(cli.BoolTFlag{
			Name:        "secure",
			Usage:       "Verify the server's SSL certificate hostname (default true, disable by --secure=false)",
			Destination: &config.Secure,
			EnvVar:      "CLISIANA_VERIFY_SSL",
		}),
		altsrc.NewBoolTFlag(cli.BoolTFlag{
			Name:        "history",
			Usage:       "Keep persistent readline history (default true, disable by --history=false)",
			Destination: &config.RLHistory,
			EnvVar:      "CLISIANA_HISTORY",
		}),
		altsrc.NewStringFlag(cli.StringFlag{
			Name:        "history-file",
			Value:       config.XDGApp.DataPath("readline-history"),
			Usage:       "The readline history file to write to",
			Destination: &config.RLHistoryFile,
			EnvVar:      "CLISIANA_HISTORY_FILE",
		}),
		altsrc.NewStringFlag(cli.StringFlag{
			Name:        "prompt-color",
			Usage:       "Prompt colour, one of green (default), black, red, yellow, blue, magenta, cyan, white or none",
			Value:       "green",
			Destination: &config.PromptColor,
			EnvVar:      "CLISIANA_COLOR",
		}),
		altsrc.NewStringFlag(cli.StringFlag{
			Name:        "prompt",
			Value:       "🌷",
			Usage:       "The readline prompt to use (without trailing space)",
			Destination: &config.Prompt,
			EnvVar:      "CLISIANA_PROMPT",
		}),
		altsrc.NewStringFlag(cli.StringFlag{
			Name:        "cache-file",
			Value:       config.XDGApp.CachePath("message-cache.sqlite3"),
			Usage:       "The message cache file to write to",
			Destination: &config.CacheFile,
			EnvVar:      "CLISIANA_CACHE_FILE",
		}),
		altsrc.NewBoolFlag(cli.BoolFlag{
			Name:        "logging",
			Usage:       "Enable local logging of messages",
			Destination: &config.Logging,
			EnvVar:      "CLISIANA_ENABLE_LOGGING",
		}),
		altsrc.NewStringFlag(cli.StringFlag{
			Name:        "log-file",
			Value:       config.XDGApp.DataPath("message-log.sqlite3"),
			Usage:       "The message log file to write to (SQLite3 format)",
			Destination: &config.LogFile,
			EnvVar:      "CLISIANA_LOG_FILE",
		}),
	}

	cliApp.Before = func(context *cli.Context) error {
		err := altsrc.InitInputSourceWithContext(config.CLIApp.Flags, configFileFromFlags)(context)
		if err != nil {
			return fmt.Errorf("Specified configuration file could not be read (%s)", strings.TrimSuffix(err.Error()[59:], "'"))
		}
		return nil
	}
	return *cliApp
}

// This is just a wrapper on altsrc.NewYamlSourceFromFlagFunc("config") with nicer error messages
func configFileFromFlags(context *cli.Context) (altsrc.InputSourceContext, error) {
	// Try to stat the file
	fi, err := os.Stat(context.String("config"))
	if err != nil {
		if context.IsSet("config") {
			// Only print an error message if the config file was specified (i.e. is not default)
			return nil, fmt.Errorf("stat failed")
		}
		return nil, nil // silently fail ¯\_(ツ)_/¯
	}

	// Make sure it's a regular file
	if fi.Mode().IsRegular() {
		// Open in RDONLY mode...
		f, err := os.Open(context.String("config"))
		if err != nil {
			if context.IsSet("config") {
				// Only print an error message if the config file was specified (i.e. is not default)
				return nil, fmt.Errorf("open failed")
			}
			return nil, nil // silently fail ¯\_(ツ)_/¯
		}
		f.Close()
		return func(ctx *cli.Context) (altsrc.InputSourceContext, error) {
			isc, err := altsrc.NewYamlSourceFromFlagFunc("config")(ctx)
			if err != nil {
				return nil, fmt.Errorf("invalid YAML?")
			}
			return isc, nil
		}(context)
	}

	// Not a regular file
	if context.IsSet("config") {
		// Only print an error message if the config file was specified (i.e. is not default)
		return nil, fmt.Errorf("not a regular file")
	}
	return nil, nil
}
