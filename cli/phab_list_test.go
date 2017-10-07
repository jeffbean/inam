package main

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/etcinit/gonduit/entities"
	"github.com/etcinit/gonduit/responses"
	"github.com/etcinit/gonduit/test/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPhabCommandTasks(t *testing.T) {
	tests := []struct {
		responses  map[string]interface{}
		tasks      string
		serverCode int
		wantOut    string
		wantErr    string
	}{
		{
			wantOut:    "Task: T123456 - status: closed\n",
			tasks:      "T123456",
			serverCode: http.StatusOK,
			responses: map[string]interface{}{
				"result": responses.PHIDLookupResponse{
					"T123456": &entities.PHIDResult{
						Name:   "T123456",
						PHID:   "phid-testing-123",
						Status: "closed",
					},
				},
			},
		},
		{
			wantErr:    "task not found: T123456",
			tasks:      "T123456",
			serverCode: http.StatusOK,
			responses: map[string]interface{}{
				"result": responses.PHIDLookupResponse{},
			},
		},
		{
			wantErr:    "1234: some error from conduit",
			tasks:      "T123456",
			serverCode: http.StatusOK,
			responses: map[string]interface{}{
				"result":     responses.PHIDLookupResponse{},
				"error_code": "1234",
				"error_info": "some error from conduit",
			},
		},
		{
			wantOut:    "Task: T123 - status: closed\nTask: T456 - status: open\n",
			tasks:      "T123,T456",
			serverCode: http.StatusOK,
			responses: map[string]interface{}{
				"result": responses.PHIDLookupResponse{
					"T123": &entities.PHIDResult{
						Name:   "T123",
						PHID:   "phid-testing-123",
						Status: "closed",
					},
					"T456": &entities.PHIDResult{
						Name:   "T456",
						PHID:   "phid-testing-456",
						Status: "open",
					},
				},
			},
		},
		{
			wantOut:    "Task: A023 - status: closed\nTask: T123 - status: closed\nTask: T456 - status: open\n",
			tasks:      "T123,T456",
			serverCode: http.StatusOK,
			responses: map[string]interface{}{
				"result": responses.PHIDLookupResponse{
					"T123": &entities.PHIDResult{
						Name:   "T123",
						PHID:   "phid-testing-123",
						Status: "closed",
					},
					"T456": &entities.PHIDResult{
						Name:   "T456",
						PHID:   "phid-testing-456",
						Status: "open",
					},
					"A023": &entities.PHIDResult{
						Name:   "A023",
						PHID:   "phid-testing-123",
						Status: "closed",
					},
				},
			},
		},
	}
	logger := zap.NewNop()
	outputBuf := &bytes.Buffer{}

	for _, tt := range tests {
		t.Run(tt.wantErr, func(t *testing.T) {
			outputBuf.Reset()

			s := server.New()
			defer s.Close()

			s.RegisterCapabilities()

			s.RegisterMethod(
				"phid.lookup",
				tt.serverCode,
				tt.responses,
			)

			baseCmd := newPhabListCommand(&options{}, logger)
			assert.Equal(t, "phab", baseCmd.Name())

			cmd, ok := baseCmd.(*phabCommand)
			require.True(t, ok, "conversion to phabCommand failed")

			cmd.output = outputBuf
			cmd.opts.Verbose = true
			cmd.PhabURI = s.GetURL()
			cmd.PhabAPIToken = "some-token"
			cmd.Tasks = tt.tasks

			err := cmd.Execute(nil /* args */)
			if len(tt.wantErr) > 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantOut, outputBuf.String())
			}
		})
	}
}
func TestPhabCommandProjects(t *testing.T) {
	tests := []struct {
		responses map[string]interface{}
		projects  string
		wantOut   string
		wantErr   string
		wantLogs  []observer.LoggedEntry
	}{
		{
			wantOut:  "Project: Hello World",
			projects: "Hello World",
			responses: map[string]interface{}{
				"result": responses.ProjectQueryResponse{
					Data: map[string]entities.Project{
						"phid-bah": entities.Project{
							Name: "Hello World",
							PHID: "phid-bah",
						},
					},
				},
			},
		},
		{
			wantOut: "Project: Test project",
			wantLogs: []observer.LoggedEntry{{
				Entry:   zapcore.Entry{Level: zap.ErrorLevel, Message: "errors looking up projects"},
				Context: []zapcore.Field{zap.Error(errors.New("project not found: Dan testing"))},
			}},
			projects: "Test project,Dan testing",
			responses: map[string]interface{}{
				"result": responses.ProjectQueryResponse{
					Data: map[string]entities.Project{
						"phid-bah": entities.Project{
							Name: "Test project",
							PHID: "phid-bah",
						},
					},
				},
			},
		},
		{
			wantLogs: []observer.LoggedEntry{{
				Entry:   zapcore.Entry{Level: zap.ErrorLevel, Message: "errors looking up projects"},
				Context: []zapcore.Field{zap.Error(errors.New("project not found: Test project"))},
			}},
			projects: "Test project",
			responses: map[string]interface{}{
				"result": responses.ProjectQueryResponse{},
			},
		},
	}
	logcore, obsLogs := observer.New(zap.InfoLevel)
	logger := zap.New(logcore)
	outputBuf := &bytes.Buffer{}

	for _, tt := range tests {
		t.Run(tt.wantErr, func(t *testing.T) {
			outputBuf.Reset()

			s := server.New()
			defer s.Close()

			s.RegisterCapabilities()

			s.RegisterMethod(
				"project.query",
				http.StatusOK,
				tt.responses,
			)

			baseCmd := newPhabListCommand(&options{}, logger)
			assert.Equal(t, "phab", baseCmd.Name())

			cmd, ok := baseCmd.(*phabCommand)
			require.True(t, ok, "conversion to phabCommand failed")

			cmd.output = outputBuf
			cmd.opts.Verbose = true
			cmd.PhabURI = s.GetURL()
			cmd.PhabAPIToken = "some-token"
			cmd.Projects = tt.projects

			err := cmd.Execute(nil /* args */)

			if len(tt.wantErr) > 0 {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantOut, outputBuf.String())
			if len(tt.wantLogs) > 0 {
				assert.Equal(t, tt.wantLogs, obsLogs.AllUntimed())
			}
			// Truncate the observer logs
			obsLogs.TakeAll()
		})
	}
}

func TestPhabCommandArgs(t *testing.T) {
	tests := []struct {
		phabURI           string
		phabAPIToken      string
		wantErr           error
		containsErrString string
	}{
		{
			phabURI:      "asdf",
			phabAPIToken: "",
			wantErr:      errNoAPIToken,
		},
		{
			phabURI:           "http://127.0.0.1:0",
			phabAPIToken:      "not-used-api-token",
			containsErrString: "Post http://127.0.0.1:0/api/conduit.getcapabilities: dial tcp 127.0.0.1:0",
		},
	}

	for _, tt := range tests {
		logger := zap.NewNop()
		baseCmd := newPhabListCommand(&options{}, logger)
		assert.Equal(t, "phab", baseCmd.Name())

		cmd, ok := baseCmd.(*phabCommand)
		require.True(t, ok, "conversion to phabCommand failed")

		outputBuf := &bytes.Buffer{}
		cmd.output = outputBuf
		cmd.opts.Verbose = true
		cmd.PhabURI = tt.phabURI
		cmd.PhabAPIToken = tt.phabAPIToken

		err := cmd.Execute(nil /* args */)

		require.Error(t, err)
		if tt.wantErr != nil {
			assert.Equal(t, tt.wantErr, err)
		}
		if tt.containsErrString != "" {
			assert.Contains(t, err.Error(), tt.containsErrString)
		}
	}
}
