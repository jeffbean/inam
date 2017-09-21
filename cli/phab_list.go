package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/etcinit/gonduit"
	"github.com/etcinit/gonduit/core"
	"github.com/etcinit/gonduit/requests"
	"github.com/etcinit/gonduit/responses"

	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var (
	errNoDepTasksFound = errors.New("no dependency tasks found in the graph")
	errNoAPIToken      = errors.New("an api token is required to run the phab command")
)

type phabCommand struct {
	baseCommand

	PhabURI      string `long:"phab-uri" description:"The base phab uri" default:"https://phab.example.com"`
	PhabAPIToken string `long:"api-token" description:"The phab api token to connect with, https://phab.example.com/settings/user/<user>/page/apitokens/"`

	Tasks string `long:"tasks" description:"Comma sep List of tasks "`

	output io.Writer
	// The phab conduit client for the command to share the client session
	client *gonduit.Conn
}

func newPhabListCommand(opts *options, logger *zap.Logger) command {
	return &phabCommand{
		baseCommand: newBaseCommand(
			"phab",
			"Interact with Phabricator",
			"Interact with Phabricator",
			opts, logger),
		output: os.Stdout,
	}
}

func (pc *phabCommand) Execute(_ []string) error {
	if len(pc.PhabAPIToken) == 0 {
		return errNoAPIToken
	}
	// all actions in the conduit API need the PHID from the system
	//   we can lookup the PHID based on the entity in the case of a task is in the form TXXXXX
	client, err := gonduit.Dial(
		pc.PhabURI,
		&core.ClientOptions{
			APIToken: pc.PhabAPIToken,
		},
	)
	if err != nil {
		return err
	}
	pc.client = client

	tasks, err := pc.phabLookupPHIDByName(strings.Split(pc.Tasks, ","))
	if err != nil {
		return err
	}
	for _, task := range tasks {
		fmt.Fprintf(pc.output, "Task: %s - status: %s\n", task.Name, task.Status)
	}

	return nil
}

func (pc *phabCommand) phabLookupPHIDByName(tasks []string) (responses.PHIDLookupResponse, error) {
	var err error

	// This supplies a list of task ids and avoids doing a lookup per Task
	res, err := pc.client.PHIDLookup(requests.PHIDLookupRequest{
		Names: tasks,
	})
	if err != nil {
		return nil, err
	}

	// Check that we found all the tasks we were looking for.
	var errs []error
	for _, task := range tasks {
		if _, ok := res[task]; !ok {
			errs = append(errs, fmt.Errorf("task not found: %s", task))
		}
	}
	if len(errs) > 0 {
		return nil, multierr.Combine(errs...)
	}

	return res, multierr.Combine(errs...)
}
