package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jeffbean/inam/phab"

	"github.com/etcinit/gonduit"
	"github.com/etcinit/gonduit/core"
	"github.com/etcinit/gonduit/entities"
	"github.com/etcinit/gonduit/requests"
	"github.com/etcinit/gonduit/responses"
	"github.com/pkg/errors"
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

	TasksByOwner string `long:"task_author" description:"query for phab tasks by owner"`

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

	if len(pc.Projects) > 0 {
		projects, err := phabProjectLookup(pc.client, strings.Split(pc.Projects, ","))
		if err != nil {
			return err
		}
		// We dont want to fail but just log some errors on if we cant find what we were looking for
		if err = compareProjects(strings.Split(pc.Projects, ","), projects); err != nil {
			pc.logger.Error("errors looking up projects", zap.Error(err))
		}

		for projectName, p := range projects {
			fmt.Fprintf(pc.output, "Project: %s\n", projectName)
			pc.logger.Debug("project found", zap.Any("phid", p.PHID), zap.Any("name", p.Name))
		}

		if len(projects) > 0 {
			var projectLookup []string
			// Now search for all manifests for the projects we found.
			for _, project := range projects {
				projectLookup = append(projectLookup, project.PHID)
			}
			tasks, err := pc.phabManiphestQueryTree(requests.ManiphestQueryRequest{
				ProjectPHIDs: projectLookup,
				Status:       "status-open",
			})
			if err != nil {
				return err
			}
			sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
			for _, task := range tasks {
				fmt.Fprintf(pc.output, phab.StringTree(task))
			}
		}
	}

	if len(pc.Tasks) > 0 {
		phids, err := pc.phabLookupPHIDByName(strings.Split(pc.Tasks, ","))
		if err != nil {
			return err
		}
		for _, result := range phids {
			taskList = append(taskList, result)
			// FIXME: this is showing the tree and the list of tasks -
			// pick one and change this all around
			if result.Type == "TASK" {
				tasks, err := pc.phabManiphestQueryTree(requests.ManiphestQueryRequest{
					PHIDs:  []string{result.PHID},
					Status: "status-open",
				})
				if err != nil {
					return fmt.Errorf("failed to get task from phab ids: %v", err)
				}
				sort.Slice(tasks, func(i, j int) bool { return tasks[i].Status < tasks[j].Status })
				for _, task := range tasks {
					fmt.Fprintf(pc.output, phab.StringTree(task))
				}
			}
		}

		sort.Slice(taskList, func(i, j int) bool { return taskList[i].PHID < taskList[j].PHID })
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

func (pc *phabCommand) phabManiphestQueryTree(req requests.ManiphestQueryRequest) ([]*phab.TaskTree, error) {
	// var errs []error
	var items []*phab.TaskTree

	res, err := pc.client.ManiphestQuery(req)
	if err != nil {
		return nil, err
	}

	for _, task := range *res {
		child := &phab.TaskTree{ManiphestTask: task, Items: nil}

		if len(task.DependsOnTaskPHIDs) > 0 {
			pc.logger.Debug("task has dependant tasks", zap.Any("task", task.ObjectName), zap.Any("items", task.DependsOnTaskPHIDs))
			newTasks, err := pc.phabManiphestQueryTree(requests.ManiphestQueryRequest{
				PHIDs:  task.DependsOnTaskPHIDs,
				Status: "status-open",
			})
			if err != nil {
				return nil, err
			}

			child.Items = newTasks
		}
		items = append(items, child)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func phabProjectLookup(client *gonduit.Conn, projects []string) (map[string]*entities.Project, error) {
	projectMap := make(map[string]*entities.Project)
	if len(projects) < 1 {
		return projectMap, nil
	}

	var err error
	// we should only get at least the length of the searched values back

	// This supplies a list of project names to avoids doing a lookup per Project
	res, err := client.ProjectQuery(requests.ProjectQueryRequest{
		Names: projects,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find projects: %v", projects)
	}

	for _, project := range res.Data {
		projectMap[project.Name] = &project
	}

	return projectMap, nil
}

func compareProjects(projects []string, foundProjects map[string]*entities.Project) error {
	// Check that we found all the projects we were looking for.
	var errs []error
	for _, project := range projects {
		if _, ok := foundProjects[project]; !ok {
			errs = append(errs, fmt.Errorf("project not found: %s", project))
		}
	}
	return multierr.Combine(errs...)
}
