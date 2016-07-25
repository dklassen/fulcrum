package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
)

var (
	client              = http.Client{}
	apiToken            = ""
	baseURI             = "api.lever.co/v1/"
	registeredEndpoints = map[string]Endpoint{
		"downloadUsers": Endpoint{
			Name:        "Download Users",
			Method:      "GET",
			Handler:     DownloadUsers,
			SprintfPath: "/users",
			Description: "Download all users from lever.",
		},
		"downloadInterviews": Endpoint{
			Name:        "Download Interviews",
			Method:      "GET",
			Handler:     DownloadInterviews,
			SprintfPath: "/candidates/%s/interviews",
			Description: "Download interviews for a candidates",
		},
		"downloadFeedback": Endpoint{
			Name:        "Download Feedback",
			Method:      "GET",
			Handler:     DownloadCandidateFeedback,
			SprintfPath: "/candidates/%s/feedback",
			Description: "Download feedback for a candidates",
		},
		"downloadCandidates": Endpoint{
			Name:        "Download Candidates",
			Method:      "GET",
			Handler:     DownloadCandidates,
			SprintfPath: "/candidates",
			Description: "Download all candidates",
		},
	}
)

type Endpoint struct {
	Name        string
	Type        string
	Method      string
	Handler     func(endpoint Endpoint, input string) error
	Data        *strings.Reader
	SprintfPath string
	Description string
	Arguments   []interface{} // TODO:: rename this sucker to something that reflects is used in the sprintf for things like candidate id's
	QueryParams []QueryParam
}

type Candidates struct {
	Data    []Candidate `json:"data"`
	Next    string      `json:"next"`
	HasNext bool        `json:"hasNext"`
}

type Candidate struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	CreatedAt  int      `json:"createdAt"`
	ArchivedAt int      `json:"archivedAt"`
	Tags       []string `json:"tags"`
}

type Users struct {
	Data    []User `json:"data"`
	Next    string `json:"next"`
	HasNext bool   `json:"hasNext"`
}

// User in Lever include any team member that has been invited to join in on recruiting efforts.
// There are five different access roles in Lever. From greatest access to least,
// these roles are: Super Admin, Admin, Team Member, Team Member - Limited, and Interviewer.
type User struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Username   string `json:"username"`
	Email      string `json:"username"`
	CreatedAt  int    `json:"createdAt"`
	AccessRole string `json:"accessRole"`
}

type Feedbacks struct {
	Data    []Feedback `json:"data"`
	Next    string     `json:"next"`
	HasNext bool       `json:"hasNext"`
}

type Feedback struct {
	ID             string      `json:"id"`
	Type           string      `json:"type"`
	Text           string      `json:"text"`
	Instructions   string      `json:"instructions"`
	Fields         []FormField `json:"fields"`
	BaseTemplateID string      `json:"baseTemplateId"`
	Interview      string      `json:"interview"`
	User           string      `json:"user"`
	CreatedAt      int         `json:"createdAt"`
	CompletedAt    int         `json:"completedAt"`
}

type FormField struct {
	Type        string      `json:"type"`
	Text        string      `json:"text"`
	Value       interface{} `json:"value"`
	Description string      `json:"Description"`
	Required    bool        `json:"required"`
}

type Interviews struct {
	Data    []Interview `json:"data"`
	Next    string      `json:"next"`
	HasNext bool        `json:"hasNext"`
}

type Interview struct {
	ID               string   `json:"id"`
	Subject          string   `json:"subject"`
	Note             string   `json:"note"`
	Interviewers     []User   `json:"interviewers"`
	Timezone         string   `json:"timezone"`
	Date             int      `json:"date"`
	Duration         int      `json:"duration"`
	Location         string   `json:"location"`
	FeedbackTemplate string   `json:"feedbackTemplate"`
	FeedbackForms    []string `json:"feedbackForms"`
	User             string   `json:"user"`
	Stage            string   `json:"stage"`
	CanceledAt       int      `json:"canceledAt"`
}

type QueryParam struct {
	Field string
	Value string
}

func (endpoint *Endpoint) PartialPath() string {
	return path.Join(baseURI, endpoint.SprintfPath)
}

// URL create an endpoint url substituting any required path segments
func (endpoint *Endpoint) URL() *url.URL {
	result := fmt.Sprintf(endpoint.PartialPath(), endpoint.Arguments...)
	endpointURL, err := url.Parse(result)
	if err != nil {
		logrus.Fatal("Unable to process endpoint arguments: ", err)
	}
	endpointURL.Scheme = "https"
	return endpointURL
}

// URLString returns a string representation of the URL for the endpoint
func (endpoint *Endpoint) URLString() string {
	u := endpoint.URL()
	for _, param := range endpoint.QueryParams {
		q := u.Query()
		q.Set(param.Field, param.Value)
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// ExecuteLeverRequest against an endpoint and decode the json into the
// passed in object
func ExecuteLeverRequest(endpoint Endpoint, v interface{}) error {
	req, err := http.NewRequest(endpoint.Method, endpoint.URLString(), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(apiToken, "")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		logrus.Error("Non 200 HTTP status response from ", endpoint.URLString())
		logrus.Fatal(resp)
	}

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, &v)
	if err != nil {
		return err
	}
	return nil
}

// DownloadCandidateFeedback reads a CSV of candidate id's and downloads any
// and all feedback for the individual
func DownloadCandidateFeedback(endpoint Endpoint, input string) error {
	if input == "" {
		logrus.Fatal("To download feedback we need a csv file with a list of candidate ids.")
	}

	f, err := os.Open(input)
	if err != nil {
		logrus.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	enc := json.NewEncoder(os.Stdout)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			logrus.Fatal(err)
		}

		// NOTE:: Sucks but we need to know that this endpoint requires specific arguments
		endpoint.Arguments = []interface{}{record[0]}

		var feedbacks Feedbacks
		if err := ExecuteLeverRequest(endpoint, &feedbacks); err != nil {
			return err
		}

		for _, feedback := range feedbacks.Data {
			if err := enc.Encode(&feedback); err != nil {
				logrus.Error(err)
			}
		}
	}
	return nil
}

// DownloadInterviews reads candidate ids from a csv file and grabs the interviews
// for that candidate
// NOTE:: This is going to bite me in the ass. We are not going to try and download
// all the interviews. Grap the first list and thats it. If someone has more than 50
// interviews seriously will worry about it then
func DownloadInterviews(endpoint Endpoint, input string) error {
	if input == "" {
		logrus.Fatal("To download interviews we need a csv file with a list of candidate ids.")
	}

	f, err := os.Open(input)
	if err != nil {
		logrus.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	enc := json.NewEncoder(os.Stdout)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.Fatal(err)
		}

		endpoint.Arguments = []interface{}{record[0]}

		var interviews Interviews
		err = ExecuteLeverRequest(endpoint, &interviews)
		if err != nil {
			return err
		}

		for _, interview := range interviews.Data {
			if err := enc.Encode(&interview); err != nil {
				logrus.Error(err)
			}
		}
	}
	return nil
}

// DownloadUsers downloads the json formatted list of all users
// that are involved in giving interviews on lever
func DownloadUsers(endpoint Endpoint, input string) error {
	u := endpoint.URL()
	for {
		var users Users
		ExecuteLeverRequest(endpoint, &users)
		if !users.HasNext {
			break
		}

		// Update the offset and grab the next list of users
		q := u.Query()
		q.Set("offset", users.Next)
		u.RawQuery = q.Encode()

		enc := json.NewEncoder(os.Stdout)

		for _, user := range users.Data {
			if err := enc.Encode(&user); err != nil {
				logrus.Error(err)
			}
		}
	}
	return nil
}

// DownloadCandidates retrieve candidates from lever
func DownloadCandidates(endpoint Endpoint, input string) error {
	u := endpoint.URL()
	for {
		var candidates Candidates
		err := ExecuteLeverRequest(endpoint, &candidates)
		if err != nil {
			return err
		}

		if !candidates.HasNext {
			break
		}

		// Update the offset and grab the next list of users
		q := u.Query()
		q.Set("offset", candidates.Next)
		u.RawQuery = q.Encode()

		enc := json.NewEncoder(os.Stdout)

		for _, candidate := range candidates.Data {
			if err := enc.Encode(&candidate); err != nil {
				logrus.Error(err)
			}
		}
	}
	return nil
}
