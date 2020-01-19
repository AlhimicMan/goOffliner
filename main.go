package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/client"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

func main() {

	if !CheckArgs(os.Args) {
		fmt.Println("Errors in arguments \"<url>\", \"<directory>\", \"<proxy URL>\"")
		return
	}

	err := CheckProxy(os.Args)
	if err != nil {
		fmt.Printf("Cant set custom proxy settings %s", os.Args[3])
	}

	urlToClone := os.Args[1]
	fmt.Println("git clone " + urlToClone)

	directory := os.Args[2]

	repoPath := getRepo(urlToClone, directory)

	if len(repoPath) > 0 {
		resultStr := fmt.Sprintf("Downloaded %s successfully", urlToClone)
		fmt.Println(resultStr)
	}
	// repoPath = "/Users/borispahomov/go/goOffline/github.com/google/gopacket"
	depDownloaded := downloadDependencies(repoPath, directory)
	if depDownloaded == nil {
		fmt.Println("error resolving dependencies", depDownloaded)
	}
}

func setProxy(proxyURLStr string) error {
	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		return err
	}
	// Create a custom http(s) client with your config
	customClient := &http.Client{
		// accept any certificate (might be useful for testing)
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           http.ProxyURL(proxyURL),
		},

		// 15 second timeout
		Timeout: 15 * time.Second,

		// don't follow redirect
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Override http(s) default protocol to use our custom client
	client.InstallProtocol("https", githttp.NewClient(customClient))
	return nil
}

func getRepo(urlToClone, directory string) string {
	targetPath, dirErr := makeTargetDirStructure(urlToClone, directory)
	if dirErr != nil {
		fmt.Println(dirErr)
		return ""
	}

	r, err := git.PlainClone(targetPath, false, &git.CloneOptions{
		URL:               urlToClone,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	fmt.Println(r)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return targetPath
}

func downloadDependencies(repoPath, directory string) error {
	dependencies, err := findDependencies(repoPath)
	if err != nil {
		return err
	}
	for _, depURL := range dependencies {
		repoPath := getRepo(depURL, directory)
		if len(repoPath) > 0 {
			resultStr := fmt.Sprintf("Downloaded %s successfully", depURL)
			fmt.Println(resultStr)
		}
		depDownloaded := downloadDependencies(repoPath, directory)
		if depDownloaded != nil {
			fmt.Printf("Error downloading dependencies for %s\n", repoPath)
		}
	}
	return nil
}

func findDependencies(repoPath string) ([]string, error) {
	modFilePath := repoPath + "/go.mod"
	modFile, err := os.Open(modFilePath)
	var dependencies []string
	if err != nil {
		return dependencies, err
	}

	defer modFile.Close()

	scanner := bufio.NewScanner(modFile)
	requireFound := false
	downloadableDepReg := regexp.MustCompile(`[a-zA-z0-9]+\.[a-zA-Z0-9]+/.+\sv\d`)
	for scanner.Scan() {
		fLine := scanner.Text()
		if requireFound {
			if strings.Index(fLine, ")") != -1 {
				break
			}
			findStr := downloadableDepReg.FindAllString(fLine, 1)
			if len(findStr) > 0 {
				repoURL := "https://" + strings.TrimSpace(findStr[0])
				repoURL = repoURL[:len(repoURL)-3]
				dependencies = append(dependencies, repoURL)
				fmt.Printf("Dependency %s\n", repoURL)
			}
		}
		if strings.Index(fLine, "require") != -1 {
			requireFound = true
		}

	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return dependencies, err
}

// exists returns whether the given file or directory exists
func dirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// CheckArgs - Checking arguments
func CheckArgs(args []string) bool {
	// "<url>", "<directory>"
	if len(args) < 3 {
		return false
	}
	urlPattern := regexp.MustCompile("https{0,1}://.*")
	urlValid := urlPattern.MatchString(args[1])
	if !urlValid {
		return false
	}
	return true
}

//CheckProxy checks for proxy settings in command line
func CheckProxy(args []string) error {
	if len(args) == 4 {
		err := setProxy(args[3])
		if err != nil {
			return err
		}
	}
	return nil
}

func makeTargetDirStructure(url string, rootDir string) (string, error) {
	urlProtocol := regexp.MustCompile("https{0,1}://")
	pathStr := urlProtocol.ReplaceAllString(url, "")
	pathStruct := strings.Split(pathStr, "/")
	targetPath := rootDir + "/"
	for _, dir := range pathStruct {
		targetPath = targetPath + "/" + dir
		if status, _ := dirExists(targetPath); !status {
			err := os.Mkdir(targetPath, 0777)
			if err != nil {
				return "", err
			}
		}
	}
	return targetPath, nil
}
