package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"text/template"

	"github.com/jeffbean/inam/phab"

	"github.com/etcinit/gonduit"
	"github.com/etcinit/gonduit/core"
	"github.com/etcinit/gonduit/entities"
	"github.com/etcinit/gonduit/requests"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

var (
	errNoUsersSpecified   = errors.New("no users specified to create task")
	errNoProjectSpecified = errors.New("no project specified to create task")
)

// ManiphestCreateTaskRequest represents a request to maniphest.createtask.
type ManiphestCreateTaskRequest struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	OwnerPHID    string   `json:"ownerPHID"`
	ViewPolicy   string   `json:"viewPolicy"`
	EditPolicy   string   `json:"editPolicy"`
	CCPHIDs      []string `json:"ccPHIDs"`
	Priority     int      `json:"priority"`
	ProjectPHIDs []string `json:"projectPHIDs"`
	Auxiliary    Aux      `json:"auxiliary"`
	requests.Request
}

type Aux struct {
	Type string `json:"std:maniphest:task_type"`
}

type yamlConfig struct {
	TaskTemplate  string `yaml:"taskTemplate"`
	TitleTemplate string `yaml:"titleTemplate"`

	CommonProjects []string `yaml:"commonProjects" `
	CommonCCUsers  []string `yaml:"commonCCUsers" `

	Emails []emailConfig `yaml:"emails"`
}

type emailConfig struct {
	Owner    string   `yaml:""`
	CCUsers  []string `yaml:",omitempty"`
	Projects []string `yaml:",omitempty"`
	// InsertHere is aninterface you can then use in the template as you please
	InsertHere string `yaml:"insertHere,omitempty"`
}

type phabBulkCreateCommand struct {
	baseCommand

	PhabURI      string `long:"phab-uri" description:"The base phab uri" default:"https://phab.example.com"`
	PhabAPIToken string `long:"api-token" description:"The phab api token to connect with, https://phab.example.com/settings/user/<user>/page/apitokens/"`

	EmailConfig string `long:"email-config" description:"The yaml file configuring the bulk emails"`

	ActuallyCreate bool `long:"actually-create" description:"The default action is to dry run the action and skip the actual task create. It will log what it intends to do."`

	output io.Writer
	// The phab conduit client for the command to share the client session
	client *gonduit.Conn
}

func newPhabBulkCreateCommand(opts *options, logger *zap.Logger) command {
	return &phabBulkCreateCommand{
		baseCommand: newBaseCommand(
			"phab-bulk-create",
			"Create a set of tasks with a list of users.",
			"Using a template to fill in the description and title you can create a mass amount of tasks for various reasons.",
			opts, logger),
		output: os.Stdout,
	}
}

func (pc *phabBulkCreateCommand) Execute(_ []string) error {
	// Parse in the config yaml
	yamlFile, err := ioutil.ReadFile(pc.EmailConfig)
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}

	conf := yamlConfig{}
	if err := yaml.Unmarshal(yamlFile, &conf); err != nil {
		log.Fatalf("failed to unmarshal config fle: %v", err)
	}

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

	projects := make(map[string]*entities.Project)
	if projects, err = phabProjectLookup(pc.client, conf.CommonProjects); err != nil {
		return err
	}

	// We dont want to fail but just log some errors on if we cant find what we were looking for
	if err = compareProjects(conf.CommonProjects, projects); err != nil {
		pc.logger.Error("errors looking up projects", zap.Error(err))
		return err
	}
	commonUsers := make(map[string]phab.User)
	// default conduit will return all users
	if commonUsers, err = getPhabUsers(pc.client, conf.CommonCCUsers); err != nil {
		return err
	}

	pc.logger.Info("common users", zap.Any("users", commonUsers))

	var errs error
	for _, emailConf := range conf.Emails {
		descriptionTemplate, err := template.New("desc" + emailConf.Owner).Parse(conf.TaskTemplate)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		titleTemplate, err := template.New("title" + emailConf.Owner).Parse(conf.TitleTemplate)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		p := createTaskParams{
			titleTemplate:       titleTemplate,
			descriptionTemplate: descriptionTemplate,
			emailConf:           emailConf,
			commonProjects:      projects,
			commonUsers:         commonUsers,
		}
		if err := pc.createTemplateTask(p); err != nil {
			pc.logger.Error("failed to create task", zap.Error(err), zap.String("owner", p.emailConf.Owner))
			errs = multierr.Append(errs, fmt.Errorf("failed for entry %q: %v", p.emailConf.Owner, err))
		}
	}

	return errs
}

type createTaskParams struct {
	titleTemplate       *template.Template
	descriptionTemplate *template.Template
	emailConf           emailConfig
	commonProjects      map[string]*entities.Project
	commonUsers         map[string]phab.User
}

func (pc *phabBulkCreateCommand) createTemplateTask(p createTaskParams) error {
	emailProjects, err := phabProjectLookup(pc.client, p.emailConf.Projects)
	if err != nil {
		return err
	}
	// We dont want to fail but just log some errors on if we cant find what we were looking for
	if err = compareProjects(p.emailConf.Projects, emailProjects); err != nil {
		pc.logger.Error("errors looking up email projects", zap.Error(err))
		return errors.Wrapf(err, "failed to find project for email config: %v", p.emailConf.Owner)
	}
	emailUsers := make(map[string]phab.User)
	if emailUsers, err = getPhabUsers(pc.client, p.emailConf.CCUsers); err != nil {
		return err
	}

	pc.logger.Debug("email users", zap.Any("users", emailUsers))

	owner, err := getPhabUsers(pc.client, []string{p.emailConf.Owner})
	if err != nil {
		return errors.Wrapf(err, "failed to find phab user: %v", p.emailConf.Owner)
	}
	pc.logger.Debug("owner user found", zap.Any("users", owner))

	if err := compareUsers([]string{p.emailConf.Owner}, owner); err != nil {
		pc.logger.Error("errors looking up owner user", zap.Error(err))
		return errors.Wrapf(err, "failed to find user for email config: %v", p.emailConf.Owner)
	}

	titleBuf := &bytes.Buffer{}
	if err := p.titleTemplate.Execute(titleBuf, p.emailConf); err != nil {
		return errors.Wrapf(err, "failed to execute title template")
	}

	descBuf := &bytes.Buffer{}
	if err := p.descriptionTemplate.Execute(descBuf, p.emailConf); err != nil {
		return errors.Wrapf(err, "failed to execute description template")
	}

	// working with what i have now...
	var allProjects []*entities.Project
	for _, p := range p.commonProjects {
		allProjects = append(allProjects, p)
	}
	for _, p := range emailProjects {
		allProjects = append(allProjects, p)
	}

	var projectPHIDs []string
	for _, p := range allProjects {
		projectPHIDs = append(projectPHIDs, p.PHID)
	}
	// just making it work now...
	var allUsers []phab.User
	for _, u := range p.commonUsers {
		allUsers = append(allUsers, u)
	}
	for _, u := range emailUsers {
		allUsers = append(allUsers, u)
	}
	// finally create a task :D
	if pc.ActuallyCreate {
		newTask, err := pc.createNewPhabTask(titleBuf.String(), descBuf.String(), projectPHIDs, owner[p.emailConf.Owner], allUsers)
		if err != nil {
			return errors.Wrapf(err, "failed to create new task for owner: %v", p.emailConf.Owner)
		}
		if len(allUsers) > 30 {
			pc.logger.Warn("more than 30 users ccd on the task", zap.String("id", newTask.ID))
		}
		pc.logger.Info("created task for user",
			zap.String("owner", p.emailConf.Owner),
			// zap.Strings("projectPHIDs", projectPHIDs),
			// zap.Any("users", allUsers),
			zap.Stringer("title", titleBuf),
			zap.String("taskID", newTask.ObjectName),
		)
		return nil
	}
	pc.logger.Info("DRY RUN",
		zap.String("owner", p.emailConf.Owner),
		zap.Any("project", allProjects),
		zap.Any("users", allUsers),
		zap.Stringer("title", titleBuf),
		zap.Stringer("description", descBuf),
	)

	return nil
}

func (pc *phabBulkCreateCommand) createNewPhabTask(
	title, description string,
	projects []string,
	owner phab.User,
	ccUsers []phab.User,
) (*entities.ManiphestTask, error) {
	if len(ccUsers) == 0 {
		return nil, errNoUsersSpecified
	}
	if len(projects) == 0 {
		return nil, errNoProjectSpecified
	}

	var ccUserIDs []string
	for _, user := range ccUsers {
		if user.PHID != "" {
			ccUserIDs = append(ccUserIDs, user.PHID)
		}
	}

	pc.logger.Debug("creating new task",
		zap.String("title", title),
		zap.String("description", description),
		zap.Any("owner", owner),
		zap.Any("ccUsers", ccUserIDs),
	)

	if !pc.ActuallyCreate {
		return &entities.ManiphestTask{ObjectName: "TFAKETASK"}, nil
	}
	var mt entities.ManiphestTask
	req := ManiphestCreateTaskRequest{
		Title:        title,
		Description:  description,
		OwnerPHID:    owner.PHID,
		ProjectPHIDs: projects,
		CCPHIDs:      ccUserIDs,
		Auxiliary:    Aux{Type: "task"},
	}
	// TODO: figure out how to specify the task type since we have custom types
	if err := pc.client.Call("maniphest.createtask", &req, &mt); err != nil {
		return nil, err
	}
	return &mt, nil
}

func compareUsers(wantUsers []string, foundUsers map[string]phab.User) error {
	// Check that we found all the projects we were looking for.
	var errs []error

	for _, user := range wantUsers {
		if _, ok := foundUsers[user]; !ok {
			errs = append(errs, fmt.Errorf("user not found: %s", user))
		}
	}

	return multierr.Combine(errs...)
}

func getPhabUsers(client *gonduit.Conn, contactUsers []string) (map[string]phab.User, error) {
	userMap := make(map[string]phab.User)
	if len(contactUsers) < 1 {
		return userMap, nil
	}

	var users phab.UserQueryResponse
	req := &phab.UserQueryRequest{Usernames: contactUsers}

	if err := client.Call("user.query", req, &users); err != nil {
		return nil, err
	}

	for _, user := range users {
		userMap[user.UserName] = user
	}

	var errs []error
	for _, user := range contactUsers {
		if _, ok := userMap[user]; !ok {
			errs = append(errs, fmt.Errorf("user not found in phab: %s", user))
		}
	}
	return userMap, multierr.Combine(errs...)
}
