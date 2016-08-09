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
	"reflect"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

var (
	client              = http.Client{}
	enc                 = json.NewEncoder(os.Stdout)
	apiToken            = ""
	baseURI             = "api.lever.co/v1/"
	registeredEndpoints = map[string]Endpoint{
		"downloadUsers": Endpoint{
			Name:        "Download Users",
			Method:      "GET",
			Type:        "users",
			Handler:     Download,
			SprintfPath: "/users",
			Description: "Download all users from lever.",
		},
		"downloadInterviews": Endpoint{
			Name:        "Download Interviews",
			Method:      "GET",
			Handler:     DownloadUsingList,
			Type:        "interviews",
			SprintfPath: "/candidates/%s/interviews",
			Description: "Download interviews for a candidates",
		},
		"downloadFeedback": Endpoint{
			Name:        "Download Feedback",
			Method:      "GET",
			Handler:     DownloadUsingList,
			Type:        "feedback",
			SprintfPath: "/candidates/%s/feedback",
			Description: "Download feedback for a candidates",
		},
		"downloadCandidates": Endpoint{
			Name:        "Download Candidates",
			Method:      "GET",
			Type:        "candidates",
			Handler:     Download,
			SprintfPath: "/candidates",
			Description: "Download all candidates",
		},
		"downloadArchivedReasons": Endpoint{
			Name:        "Download Archived Reasons",
			Method:      "GET",
			Type:        "archivedReasons",
			Handler:     Download,
			SprintfPath: "/archive_reasons",
			Description: "Download archive reasons for a candidate",
		},
		"downloadPostings": Endpoint{
			Name:        "Download Postings",
			Type:        "postings",
			Method:      "GET",
			Handler:     Download,
			SprintfPath: "/postings",
			Description: "Download all job postings",
		},
		"downloadApplications": Endpoint{
			Name:        "Download Applications",
			Type:        "applications",
			Method:      "GET",
			Handler:     DownloadUsingList,
			SprintfPath: "/candidates/%s/applications",
			Description: "Download all job applications for a candidate",
		},
	}
)

type Endpoint struct {
	Name        string
	Type        string
	Method      string
	Offset      string
	HasNext     bool
	Handler     func(endpoint Endpoint, input string) error
	Data        *strings.Reader
	SprintfPath string
	Description string
	Arguments   []interface{} // TODO:: rename this sucker to something that reflects is used in the sprintf for things like candidate id's
	QueryParams []QueryParam
}

type LeverData struct {
	Data    json.RawMessage `json:"data"`
	Next    string          `json:"next"`
	HasNext bool            `json:"hasNext"`
}

type ArchiveReason struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type Archived struct {
	ArchivedAt     int    `json:"archivedAt"`
	ArchivedReason string `json:"archivedReason"`
}

type QueryParam struct {
	Field string
	Value string
}

type Candidate struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	CreatedAt  int      `json:"createdAt"`
	ArchivedAt int      `json:"archivedAt"`
	Archived   Archived `json:"archived"`
	Tags       []string `json:"tags"`
}

type Posting struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	CreatedAt  int      `json:"createdAt"`
	UpdatedAt  int      `json:updatedAt"`
	User       string   `json:"user"`
	Owner      string   `json:"Owner"`
	Categories Category `json:"categories"`
	Tags       []string `json:"tags"`
	State      string   `json:"state"`
	ReqCode    string   `json:"reqcode"`
}

type Category struct {
	Location   string `json:"location"`
	Commitment string `json:"commitment"`
	Team       string `json:"team"`
	Level      string `json:"level"`
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

type Application struct {
	ID                   string   `json:"id"`
	CreatedAt            int      `json:"createdAt"`
	Type                 string   `json:"type"`
	Posting              string   `json:"posting"`
	PostingOwner         string   `json:"postingOwnner"`
	PostingHiringManager string   `json:"postingHiringManager"`
	User                 string   `json:"user"`
	Name                 string   `json:"name"`
	Email                string   `json:"email"`
	Company              string   `json:"company"`
	Archived             Archived `json:"archived"`
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

	if endpoint.Offset != "" {
		q := u.Query()
		q.Set("offset", endpoint.Offset)
		u.RawQuery = q.Encode()
	}

	return u.String()
}

// LeverEndpointResult is the default response object returned
// from a lever endpoint request.
type LeverEndpointResult struct {
	Data    *json.RawMessage `json:"data"`
	HasNext bool             `json:"hasNext"`
	Next    string           `json:"next"`
}

func Output(obj interface{}, encoder *json.Encoder) {
	if err := encoder.Encode(&obj); err != nil {
		logrus.Error(err)
	}
}

func OutputList(v interface{}, encoder *json.Encoder) {
	rv := reflect.ValueOf(v) //.FieldByName("Data")
	if rv.IsNil() {
		logrus.Panic("Lever JSON object must contain Data field")
	}

	for i := 0; i < rv.Len(); i++ {
		entry := rv.Index(i).Interface()
		Output(entry, enc)
	}
}

func ExecuteLeverRequest(endpoint *Endpoint, v interface{}) error {
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

	// Track next token for endpoint
	rv := reflect.ValueOf(v).Elem()
	endpoint.Offset = rv.FieldByName("Next").String()
	endpoint.HasNext = rv.FieldByName("HasNext").Bool()
	return nil
}

func DownloadUsingList(endpoint Endpoint, input string) error {
	if input == "" {
		logrus.Fatal("To download interviews we need a csv file with a list of candidate ids.")
	}

	f, err := os.Open(input)
	if err != nil {
		logrus.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.Fatal(err)
		}

		endpoint.Arguments = []interface{}{record[0]}

		for {
			var leverData LeverData
			err = ExecuteLeverRequest(&endpoint, &leverData)
			if err != nil {
				return err
			}

			// Cause we can't be downloading to fast
			time.Sleep(1000 * time.Millisecond)

			switch endpoint.Type {
			case "interviews":
				var interviews []Interview
				if err := json.Unmarshal(leverData.Data, &interviews); err != nil {
					logrus.Fatal(err)
				}

				OutputList(interviews, enc)
			case "applications":
				var applications []Application

				if err := json.Unmarshal(leverData.Data, &applications); err != nil {
					logrus.Fatal(err)
				}

				OutputList(applications, enc)
			default:
				logrus.Fatal("Unknown endpoint type: ", endpoint.Type)
			}

			if !endpoint.HasNext {
				break
			}
		}
	}
	return nil
}

func Download(endpoint Endpoint, input string) error {
	for {
		var leverData LeverData

		if err := ExecuteLeverRequest(&endpoint, &leverData); err != nil {
			return err
		}

		switch endpoint.Type {
		case "users":
			var users []User

			if err := json.Unmarshal(leverData.Data, &users); err != nil {
				logrus.Fatal(err)
			}

			OutputList(users, enc)
		case "archivedReasons":
			var reasons []ArchiveReason
			if err := json.Unmarshal(leverData.Data, &reasons); err != nil {
				logrus.Fatal(err)
			}

			OutputList(reasons, enc)
		case "postings":
			var posting []Posting
			if err := json.Unmarshal(leverData.Data, &posting); err != nil {
				logrus.Fatal(err)
			}

			OutputList(posting, enc)
		case "candidates":
			var candidates []Candidate

			if err := json.Unmarshal(leverData.Data, &candidates); err != nil {
				logrus.Fatal(err)
			}

			OutputList(candidates, enc)
		default:
			logrus.Fatal("Unknown endpoint type", endpoint.Type)
		}

		if !endpoint.HasNext {
			break
		}

	}
	return nil
}
