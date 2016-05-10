package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/casimir/xdg-go"
	"github.com/codegangsta/cli"
	"github.com/codegangsta/cli/altsrc"
	"github.com/jroimartin/gocui"
)

// Version is the application version (follows http://semver.org/)
const Version = "0.0.1"

// Config is an application configuration object
// Updates to this struct must be reflected in commandLineSetup()!
type Config struct {
	ConfigFile      string             `config-name:"config-file" yaml:"-"`
	Email           string             `config-name:"email"`
	APIKey          string             `config-name:"apikey"`
	APIBase         string             `config-name:"site"`
	Secure          bool               `config-name:"secure"`
	Prompt          string             `config-name:"prompt"`
	PromptColor     string             `config-name:"prompt-color"`
	RLHistory       bool               `config-name:"history"`
	RLHistoryFile   string             `config-name:"history-file"`
	CacheFile       string             `config-name:"cache-file"`
	Logging         bool               `config-name:"logging"`
	LogFile         string             `config-name:"log-file"`
	XDGApp          xdg.App            `config-name:"-" yaml:"-"`
	CLIApp          cli.App            `config-name:"-" yaml:"-"`
	Interface       *gocui.Gui         `config-name:"-" yaml:"-"`
	MainTextChannel chan WindowMessage `config-name:"-" yaml:"-"`
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
	// TODO: Maybe make this a reflection of the Config struct?
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
			// NB: Magic number in the prefix to be removed
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

func setConfigFromStrings(key string, value string) error {
	reflectedConfig := reflect.ValueOf(config).Elem()
	typeOfReflectedConfig := reflectedConfig.Type()
	err := fmt.Errorf("No key found matching %s", key)
	// NB: Magic constant ("-" for invisible fields)
	if key == "-" {
		return err
	}

setConfigFromStringsLoop:
	for i := 0; i < reflectedConfig.NumField(); i++ {
		if key == typeOfReflectedConfig.Field(i).Tag.Get("config-name") {
			switch reflectedConfig.Field(i).Type().Kind() {
			case reflect.String:
				reflectedConfig.Field(i).SetString(value)
				err = nil
				break setConfigFromStringsLoop
			case reflect.Bool:
				switch strings.ToLower(strings.TrimSpace(value)) {
				case "true", "t", "yes", "y", "1", "on":
					reflectedConfig.Field(i).SetBool(true)
					err = nil
					break setConfigFromStringsLoop
				case "false", "f", "no", "n", "0", "off":
					reflectedConfig.Field(i).SetBool(false)
					err = nil
					break setConfigFromStringsLoop
				default:
					err = fmt.Errorf("%s is not a valid boolean value", value)
				}
			}
		}
	}
	return err
}
