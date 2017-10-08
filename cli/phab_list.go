package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/etcinit/gonduit"
	"github.com/etcinit/gonduit/core"
	"github.com/etcinit/gonduit/entities"
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

	Tasks    string `long:"tasks" description:"Comma sep List of tasks "`
	Projects string `long:"projects" description:"Comma sep list of projects to get all tasks from"`

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
	var taskList []*entities.PHIDResult
	var taskProjectList []*entities.ManiphestTask

	if len(pc.Projects) > 0 {
		projects, err := pc.phabProjectLookup(strings.Split(pc.Projects, ","))
		if err != nil {
			return err
		}
		// We dont want to fail but just log some errors on if we cant find what we were looking for
		err = pc.compareProjects(strings.Split(pc.Projects, ","), projects)
		if err != nil {
			pc.logger.Error("errors looking up projects", zap.Error(err))
		}

		for projectName := range projects {
			fmt.Fprintf(pc.output, "Project: %s\n", projectName)
		}

		if len(projects) > 0 {
			// Now search for all manifests for the projects we found.
			for _, project := range projects {
				res, err := pc.client.ManiphestQuery(requests.ManiphestQueryRequest{
					ProjectPHIDs: []string{project.PHID},
				})
				if err != nil {
					return err
				}

				for _, result := range *res {
					taskProjectList = append(taskProjectList, result)

				}
				sort.Slice(taskProjectList, func(i, j int) bool { return taskProjectList[i].Status < taskProjectList[j].Status })
				for _, task := range taskProjectList {
					fmt.Fprintf(pc.output, "\tTask: %s - status: %-10s -> %s\n", task.ObjectName, task.Status, task.Title)
				}

			}
		}
	}

	if len(pc.Tasks) > 0 {
		tasks, err := pc.phabLookupPHIDByName(strings.Split(pc.Tasks, ","))
		if err != nil {
			return err
		}
		for _, result := range tasks {
			taskList = append(taskList, result)
		}

		sort.Slice(taskList, func(i, j int) bool { return taskList[i].Name < taskList[j].Name })
		for _, task := range taskList {
			fmt.Fprintf(pc.output, "Task: %s - status: %s\n", task.Name, task.Status)
		}
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

	return res, multierr.Combine(errs...)
}

func (pc *phabCommand) phabProjectLookup(projects []string) (map[string]*entities.Project, error) {
	var err error
	// we should only get at least the length of the searched values back
	projectMap := make(map[string]*entities.Project, len(projects))

	// This supplies a list of project names to avoids doing a lookup per Project
	res, err := pc.client.ProjectQuery(requests.ProjectQueryRequest{
		Names: projects,
	})
	if err != nil {
		return nil, err
	}

	for _, project := range res.Data {
		projectMap[project.Name] = &project
	}

	return projectMap, nil
}

func (pc *phabCommand) compareProjects(projects []string, foundProjects map[string]*entities.Project) error {
	// Check that we found all the projects we were looking for.
	var errs []error
	for _, project := range projects {
		if _, ok := foundProjects[project]; !ok {
			errs = append(errs, fmt.Errorf("project not found: %s", project))
		}
	}
	return multierr.Combine(errs...)
}
