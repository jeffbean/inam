package main

import (
	flags "github.com/jessevdk/go-flags"
	"go.uber.org/zap"
)

type command interface {
	flags.Commander

	Name() string
	ShortDescription() string
	LongDescription() string
}

type baseCommand struct {
	name      string
	shortDesc string
	longDesc  string

	opts   *options // global flags
	logger *zap.Logger
}

func newBaseCommand(name, short, long string, opts *options, logger *zap.Logger) baseCommand {
	return baseCommand{
		name:      name,
		shortDesc: short,
		longDesc:  long,
		opts:      opts,
		logger:    logger,
	}
}

// Name returns the name of a command
func (c baseCommand) Name() string {
	return c.name
}

// ShortDescription return the short description of a command
func (c baseCommand) ShortDescription() string {
	return c.shortDesc
}

// LongDescription returns the long description of a command
func (c baseCommand) LongDescription() string {
	return c.longDesc
}
