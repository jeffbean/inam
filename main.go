package main

import (
	"fmt"
	"os"

	flags "github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type options struct {
	Verbose bool                 `short:"v" long:"verbose" description:"Show verbose debug information"`
	Config  func(s string) error `long:"config" description:"INI config file" no-ini:"true"`
}

func main() {
	zapCfg := zap.NewDevelopmentConfig()
	zapCfg.Level.SetLevel(zap.InfoLevel)

	logger, err := zapCfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var opts options
	parser := flags.NewParser(&opts, flags.Default)

	opts.Config = func(s string) error {
		ini := flags.NewIniParser(parser)

		return ini.ParseFile(s)
	}

	parser.CommandHandler = func(command flags.Commander, args []string) error {
		if opts.Verbose {
			zapCfg.Level.SetLevel(zap.DebugLevel)
		}
		if command != nil {
			return command.Execute(args)
		}
		return nil
	}

	commands := []command{
		newPhabListCommand(&opts, logger),
		newPhabBulkCreateCommand(&opts, logger),
	}

	for _, cmd := range commands {
		if _, err := parser.Command.AddCommand(cmd.Name(), cmd.ShortDescription(), cmd.LongDescription(), cmd); err != nil {
			panic(err)
		}
	}

	if _, err := parser.Parse(); err != nil {
		if _, ok := errors.Cause(err).(*flags.Error); ok {
			parser.WriteHelp(os.Stdout)
		}
	}
}
