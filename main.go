package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"time"

	"github.com/coreos/pkg/flagutil"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/joho/godotenv"
)

func LoadDotEnv() {
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		log.Fatal("Error loading .env file")
	}
}

func Tweet(content string) error {
	LoadDotEnv()

	flags := flag.NewFlagSet("user-auth", flag.ExitOnError)
	consumerKey := flags.String("consumer-key", os.Getenv("API_KEY"), "Twitter Consumer Key")
	consumerSecret := flags.String("consumer-secret", os.Getenv("API_SECRET_KEY"), "Twitter Consumer Secret")
	accessToken := flags.String("access-token", os.Getenv("ACCESS_TOKEN"), "Twitter Access Token")
	accessSecret := flags.String("access-secret", os.Getenv("ACCESS_TOKEN_SECRET"), "Twitter Access Secret")
	flags.Parse(os.Args[1:])
	flagutil.SetFlagsFromEnv(flags, "TWITTER")

	if *consumerKey == "" || *consumerSecret == "" || *accessToken == "" || *accessSecret == "" {
		// log.Fatal("Consumer key/secret and Access token/secret required")
		return errors.New("Bad credentials")
	}

	config := oauth1.NewConfig(*consumerKey, *consumerSecret)
	token := oauth1.NewToken(*accessToken, *accessSecret)
	// OAuth1 http.Client will automatically authorize Requests
	httpClient := config.Client(oauth1.NoContext, token)

	// Twitter client
	client := twitter.NewClient(httpClient)

	// Verify Credentials
	verifyParams := &twitter.AccountVerifyParams{
		SkipStatus:   twitter.Bool(true),
		IncludeEmail: twitter.Bool(true),
	}
	user, _, err1 := client.Accounts.VerifyCredentials(verifyParams)
	fmt.Printf("User's ACCOUNT:\n%+v\n", user)
	if err1 != nil {
		return err1
	}

	tweet, _, err2 := client.Statuses.Update(content, nil)
	fmt.Printf("Posted Tweet\n%v\n", tweet)
	return err2
}

func ReadFile(file_name string) (string, error) {
	file, err := os.Open(file_name)
	if err != nil {
		// log.Fatal(err)
		return "", err
	}
	defer func() {
		if err = file.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	b, _ := ioutil.ReadAll(file)
	return string(b), nil
}

// TODO: this is hard to test, should be refactored
// GetFilenameFromDate(date) string
// ReadTodaysFilename(string) string
func ScheduledTweet() (string, string, error) {
	LoadDotEnv()
	current_time := time.Now()
	path := os.Getenv("FILE_PATH")
	fmt.Println(path)

	iso_date := current_time.Format("2006-Jan-02")
	full_date := current_time.Format("January 2, 2006")

	possible_files := []string{
		path + iso_date + ".md",
		path + iso_date + ".txt",
		path + full_date + ".md",
		path + full_date + ".txt",
	}
	existing_files := []string{}

	for _, file := range possible_files {
		if _, err := os.Stat(file) ; os.IsNotExist(err) {
			continue
		}
		existing_files = append(existing_files, file)
	}

	if len(existing_files) == 0 {
		return "", "", errors.New("No tweet files found")
	}

	content, err := ReadFile(existing_files[0])
	if err == nil {
		fmt.Println("Attempting to post content from: ", existing_files[0])
		return content, existing_files[0], nil
	}

	return "", "", errors.New(err.Error())
}

// TODO: either fix or remove queue system:
func IsQueueNameFormat(filename string) bool {
	// if name fits the format q-#, return true
	if string(filename[0]) == "q" && string(filename[1]) == "-" {
		return true
	}
	return false
}

// TODO: either fix or remove queue system:
func QueuedTweet() (string, string, error) {
	LoadDotEnv()
	path := os.Getenv("FILE_PATH")
	files, dir_read_error := ioutil.ReadDir(path)
	if dir_read_error != nil {
		return "", "", dir_read_error
	}
	filenames := []string{}

	for _, f := range files {
		if IsQueueNameFormat(f.Name()) {
			filenames = append(filenames, f.Name())
		}
	}

	sort.Strings(filenames)
	if len(filenames) == 0 {
		return "", "", errors.New("No queued files found")
	}
	queued_file_name := filenames[0]

	queued_file_path := path + queued_file_name

	content, read_error := ReadFile(queued_file_path)
	if read_error != nil {
		return "", "", read_error
	}

	current_time := time.Now()
	formatted_time := current_time.Format("2006-Jan-02")
	new_filename_path := path + "attempted_" + formatted_time + "_" + queued_file_name
	rename_error := os.Rename(queued_file_path, new_filename_path)
	if rename_error != nil {
		return "", "", rename_error
	}

	// return the content of the file
	return content, new_filename_path, nil
}

func main() {
	scheduled_content, scheduled_tweet_filename, scheduled_tweet_error := ScheduledTweet()
	if scheduled_tweet_error != nil {
		fmt.Println("Error scheduling file:", scheduled_tweet_filename)
		fmt.Println("Error:", scheduled_tweet_error)
	}
	post_failure := Tweet(scheduled_content)
	fmt.Println("Error posting to Twitter:", post_failure)
}
