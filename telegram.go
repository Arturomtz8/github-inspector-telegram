// This is the function that is called by google functions,
// the structure of the code must be like specified in the
// docs https://cloud.google.com/functions/docs/writing#directory-structure

package telegram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Arturomtz8/github-inspector/pkg/github"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

const (
	searchCommand          string = "/search"
	trendingCommand        string = "/trend"
	telegramApiBaseUrl     string = "https://api.telegram.org/bot"
	telegramApiSendMessage string = "/sendMessage"
	telegramTokenEnv       string = "GITHUB_BOT_TOKEN"
	defaulRepoLen          int    = 4
	repoExpr               string = `^\/search\s(([A-Za-z0-9\-\_]+))\s*.*`
	langExpr               string = `^\/search\s.*\s+lang:([\w]*)`
	authorExpr             string = `^\/search\s.*\s+author:(([A-Za-z0-9\-\_]+))`
	langParam              string = "lang"
	authorParam            string = "author"
)

const templ = `
  {{.FullName}}: {{.Description}}
  Author: {{.Owner.Login}}
  ⭐: {{.StargazersCount}}
  {{.HtmlURL}}
`

// Chat struct stores the id of the chat in question.
type Chat struct {
	Id       int    `json:"id"`
	Title    string `json:"title"`
	Username string `json:"username"`
	Type     string `json:"type"`
}

// Message struct store Chat and text data.
type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
}

// trigger deploy
// Update event.
type Update struct {
	UpdateId int     `json:"update_id"`
	Message  Message `json:"message"`
}

// Register an HTTP function with the Functions Framework
func init() {
	functions.HTTP("HandleTelegramWebhook", HandleTelegramWebhook)
}

// HandleTelegramWebhook is the web hook that has to have the handler signature.
// Listen for incoming web requests from Telegram events and
// responds back with the treding repositories on GitHub.
func HandleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	var update, err = parseTelegramRequest(r)
	if err != nil {
		log.Printf("error parsing update, %s", err.Error())
		return
	}

	log.Printf("incoming request from chat with username %s", update.Message.Chat.Username)

	// Handle multiple commands.
	switch {
	// Handle /search command to return a single repository data of interest.
	case strings.HasPrefix(update.Message.Text, searchCommand):
		// Get params from text message.
		repo, lang, author, err := ExtractParams(update.Message.Text)
		if err != nil {
			sendTextToTelegramChat(update.Message.Chat.Id, err.Error())
			fmt.Fprintf(w, "invalid input %s with error %v", update.Message.Text, err)
			return
		}

		// Get repository based off received params.
		repository, err := github.GetRepository(github.RepoURL, repo, lang, author)
		if err != nil {
			sendTextToTelegramChat(update.Message.Chat.Id, err.Error())
			fmt.Fprintf(w, "invalid input %s with error %v", update.Message.Text, err)
			return
		}

		// Parse Repo into text template.
		repoText, err := parseRepoToTemplate(repository)

		// Send responsa back to chat.
		_, err = sendTextToTelegramChat(update.Message.Chat.Id, repoText)
		if err != nil {
			sendTextToTelegramChat(update.Message.Chat.Id, err.Error())
			fmt.Fprintf(w, "invalid input %s with error %v", update.Message.Text, err)
			return
		}
		// Handle /trend command to return a list of treding repositories on GitHub.
	case strings.HasPrefix(update.Message.Text, trendingCommand):
		sanitizedString, err := sanitize(update.Message.Text, trendingCommand)
		if err != nil {
			sendTextToTelegramChat(update.Message.Chat.Id, err.Error())
			fmt.Fprintf(w, "invalid input %s with error %v", update.Message.Text, err)
			return
		}

		log.Printf("sanitized string: %s", sanitizedString)
		repos, err := github.GetTrendingRepos(github.TimeToday, sanitizedString)
		if err != nil {
			sendTextToTelegramChat(update.Message.Chat.Id, err.Error())
			fmt.Fprintf(w, "an error has ocurred, %s!", err)
			return
		}

		responseFunc, err := formatReposContentAndSend(repos, update.Message.Chat.Id)
		if err != nil {
			sendTextToTelegramChat(update.Message.Chat.Id, err.Error())
			fmt.Printf("got error %v from parsing repos", err)
			return
		}

		log.Printf("successfully distributed to chat id %d, response from loop: %s", update.Message.Chat.Id, responseFunc)
		return
	default:
		log.Printf("invalid command: %s", update.Message.Text)
		return
	}

}

// parseTelegramRequest decodes and incoming request from the Telegram hook,
// and returns an Update pointer.
func parseTelegramRequest(r *http.Request) (*Update, error) {
	var update Update

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("could not decode incoming update %s", err.Error())
		return nil, err
	}
	return &update, nil
}

// returns the term that wants to be searched or
// an string that specifies the expected input
func sanitize(s, botCommand string) (string, error) {
	var lenBotCommand int = len(botCommand)
	if len(s) >= lenBotCommand {
		if s[:lenBotCommand] == botCommand {
			s = s[lenBotCommand:]
			s = strings.TrimSpace(s)
			log.Printf("type of value entered: %T", s)
		}
	} else {
		return "", fmt.Errorf("invalid command: %s", s)

	}
	return s, nil

}

// Formats the content of the repos and uses internally sendTextToTelegramChat function
// for sending the formatted content to the respective chat
func formatReposContentAndSend(repos *github.TrendingSearchResult, chatId int) (string, error) {
	reposContent := make([]string, 0)

	// suffle the repos
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(repos.Items), func(i, j int) { repos.Items[i], repos.Items[j] = repos.Items[j], repos.Items[i] })

	for index, repo := range repos.Items {
		if index <= defaulRepoLen {
			var report = template.Must(template.New("trendinglist").Parse(templ))
			buf := &bytes.Buffer{}
			if err := report.Execute(buf, repo); err != nil {
				sendTextToTelegramChat(chatId, err.Error())
				return "", err
			}
			s := buf.String()

			reposContent = append(reposContent, s)
		}

	}

	if len(reposContent) == 0 {
		return "", errors.New("there are not trending repos yet for today, try again later")
	}

	log.Println("template created and proceeding to send repos to chat")
	log.Println("repos count to be sent", len(reposContent))

	text := strings.Join(reposContent, "\n-------------\n")
	_, err := sendTextToTelegramChat(chatId, text)
	if err != nil {
		log.Printf("error occurred publishing event %v", err)
		return "", err

	}

	return "all repos sent to chat", nil
}

// sendTextToTelegramChat sends the response from the GitHub back to the chat,
// given a chat id and the text from GitHub.
func sendTextToTelegramChat(chatId int, text string) (string, error) {
	fmt.Printf("sending %s to chat_id: %d", text, chatId)

	var telegramApi string = "https://api.telegram.org/bot" + os.Getenv("GITHUB_BOT_TOKEN") + "/sendMessage"

	response, err := http.PostForm(
		telegramApi,
		url.Values{
			"chat_id": {strconv.Itoa(chatId)},
			"text":    {text},
		})
	if err != nil {
		log.Printf("error when posting text to the chat: %s", err.Error())
		return "", err
	}
	defer response.Body.Close()
	var bodyBytes, errRead = ioutil.ReadAll(response.Body)
	if errRead != nil {
		log.Printf("error parsing telegram answer %s", errRead.Error())
		return "", err
	}

	bodyString := string(bodyBytes)
	log.Printf("body of telegram response: %s", bodyString)
	return bodyString, nil

}

// ExtractParams parse the command sent by the user and returns
// the name of the repository of interest (mandatory),
// the programming languague (if provided),
// the author of the repository (if provided).
// The structure of the command is the shown below:
// /search <repository> lang:<lang> author:<author>
// e.g.
// /search dblab lang:go author:danvergara
func ExtractParams(s string) (string, string, string, error) {
	s = strings.TrimSpace(s)

	repo, err := extractRepo(s)
	if err != nil {
		return "", "", "", err
	}

	lang, err := extractOptionalParam(s, langParam)
	if err != nil {
		return "", "", "", err
	}

	author, err := extractOptionalParam(s, authorParam)
	if err != nil {
		return "", "", "", err
	}

	return repo, lang, author, nil
}

// extractRepo returns the name of the repository,
// this is a mandatory parameter.
// This function will error out if the repos is not found.
func extractRepo(s string) (string, error) {
	repoRegexp, err := regexp.Compile(repoExpr)
	if err != nil {
		return "", err
	}

	matches := repoRegexp.FindStringSubmatch(s)

	if len(matches) >= 2 {
		return matches[1], nil
	}

	return "", fmt.Errorf("repo not found in %s", s)
}

// extractOptionalParam returns the value of the param in question.
// This function will not error out if the value is not found,
// since this kind of params is not mandatory.
func extractOptionalParam(s, param string) (string, error) {
	var matches []string
	switch param {
	case langParam:
		langRegexp, err := regexp.Compile(langExpr)
		if err != nil {
			return "", err
		}

		matches = langRegexp.FindStringSubmatch(s)

		if len(matches) >= 2 {
			return matches[1], nil
		}
	case authorParam:
		authorRegexp, err := regexp.Compile(authorExpr)
		if err != nil {
			return "", err
		}

		matches = authorRegexp.FindStringSubmatch(s)

		if len(matches) >= 2 {
			return matches[1], nil
		}
	default:
		return "", fmt.Errorf("%s option not supported", param)
	}

	// optional parameters.
	return "", nil
}

// parseRepoToTemplate returns a text parsed based on the template constant,
// to display the repository nicely to the user in the Telegram chat.
func parseRepoToTemplate(repo *github.RepoTrending) (string, error) {
	var report = template.Must(template.New("getrepo").Parse(templ))
	buf := &bytes.Buffer{}

	if err := report.Execute(buf, repo); err != nil {
		return "", err
	}

	return buf.String(), nil
}
