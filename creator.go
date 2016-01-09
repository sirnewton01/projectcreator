package main

import (
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

var userid = flag.String("user", "TestJazzAdmin1", "user ID of the admin user")
var password = flag.String("password", "TestJazzAdmin1", "password of the admin user")
var nodeploy = flag.Bool("nodeploy", false, "skip deploying process templates. Speeds up the task if you know that templates are already deployed.")
var repo = flag.String("repo", "https://localhost:9443/ccm", "repository url of the server")
var name = flag.String("name", "ProjectArea1", "name of the new project area")
var processid = flag.String("processid", "scrum2.process.ibm.com", "ID of the process template to use for this project area")
var templates = flag.Bool("templates", false, "don't create a project area, instead list the process templates that are available")
var members = flag.String("members", "TestJazzAdmin1=Team Member,TestJazzUser1=Team Member", "add members and assign roles to the project area")
var deployMTM = flag.Bool("deployMTM", false, "deploy the money that matters (jke) sample project area")

func authenticate(baseUrl string, userId string, password string) *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	cookieJar, _ := cookiejar.New(nil)
	client := &http.Client{Transport: tr, Jar: cookieJar}

	sessionReq, err := http.NewRequest("GET", baseUrl+"/oslc/workitems/1.xml", nil)
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(sessionReq)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()

	identityReq, err := http.NewRequest("GET", baseUrl+"/authenticated/identity", nil)
	if err != nil {
		panic(err)
	}
	resp, err = client.Do(identityReq)
	if err != nil {
		panic(err)
	}

	form := url.Values{}
	form.Add("j_username", userId)
	form.Add("j_password", password)

	authReq, err := http.NewRequest("POST", baseUrl+"/j_security_check", strings.NewReader(form.Encode()))
	if err != nil {
		panic(err)
	}
	authReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err = client.Do(authReq)
	if err != nil {
		panic(err)
	}

	resp.Body.Close()

	return client
}

func deployMTMSample(client *http.Client, baseJtsUrl string, baseRtcUrl string) {
	type lifecycleReq struct {
		Id               string `json:"id"`
		ApplicationUrl   string `json:"applicationUrl"`
		ApplicationTitle string `json:"applicationTitle"`
	}

	postBody := []lifecycleReq{lifecycleReq{"rtc.project.jkebanking", baseRtcUrl, "/ccm"}}
	postBodyBytes, err := json.Marshal(postBody)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", baseJtsUrl+"/lifecycle-project/templates/com.ibm.team.sample.mtm.cm", strings.NewReader(string(postBodyBytes)))
	if err != nil {
		panic(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)

	// 6.0.1 and earlier have a different URL for MTM
	if resp.Header.Get("Location") == "" {
		fmt.Printf("This server appears to be older than 6.0.2.\n")
		req, err = http.NewRequest("POST", baseJtsUrl+"/lifecycle-project/templates/com.ibm.team.sample.money.matters.ccm", strings.NewReader(string(postBodyBytes)))
		if err != nil {
			panic(err)
		}
		resp, err = client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		b, err = ioutil.ReadAll(resp.Body)
	}

	// 4.x and earlier have a different URL for deploying the MTM sample
	if resp.StatusCode == 404 {
		fmt.Printf("This server appears to be older than 5.0\n")
		req, err = http.NewRequest("POST", strings.Replace(baseJtsUrl, "/jts", "/admin", 1)+"/templates/com.ibm.team.sample.money.matters.ccm", strings.NewReader(string(postBodyBytes)))
		if err != nil {
			panic(err)
		}
		req.Header.Add("Content-Type", "application/json; charset=utf-8")

		resp, err = client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		b, err = ioutil.ReadAll(resp.Body)
	}

	if resp.StatusCode >= 300 {
		panic(string(b))
	}

	// Wait until the sample is fully deployed
	type status struct {
		State string `json:"state"`
	}

	jobUrl := resp.Header.Get("Location")
	for {
		req, err = http.NewRequest("GET", jobUrl, nil)
		if err != nil {
			panic(err)
		}

		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		b, err = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			panic(string(b))
		}
		sts := status{}
		err = json.Unmarshal(b, &sts)
		if err != nil {
			panic(err)
		}

		if sts.State == "FINISHED" {
			break
		}
		<-time.After(5 * time.Second)
	}
}

func deployProcessTemplates(client *http.Client, baseUrl string) {
	form := url.Values{}
	form.Add("owningApplicationKey", "JTS-Sentinel-Id")
	req, err := http.NewRequest("POST", baseUrl+"/service/com.ibm.team.process.internal.service.web.IProcessWebUIService/deployPredefinedProcessDefinitions", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	cookies := client.Jar.Cookies(req.URL)
	sessionId := ""
	for _, cookie := range cookies {
		if cookie.Name == "JSESSIONID" {
			sessionId = cookie.Value
		}
	}
	req.Header.Add("X-Jazz-CSRF-Prevent", sessionId)

	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		panic(string(b))
	}
}

type ProcessDefinition struct {
	Name      string `xml:"defaultName"`
	ItemId    string `xml:"itemId"`
	ProcessId string `xml:"processId"`
}

func getProcessDefinitions(client *http.Client, baseUrl string) []ProcessDefinition {
	req, err := http.NewRequest("GET", baseUrl+"/service/com.ibm.team.process.internal.service.web.IProcessWebUIService/allProcessDefinitions?owningApplicationKey=JTS-Sentinel-Id", strings.NewReader(""))
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		panic(string(b))
	}

	type ReturnValue struct {
		Values []ProcessDefinition `xml:"values"`
	}
	type Response struct {
		Method      string      `xml:"method"`
		ReturnValue ReturnValue `xml:"returnValue"`
	}
	type Body struct {
		Response Response `xml:"response"`
	}
	type ProcessDefinitionsResult struct {
		Body Body `xml:"Body"`
	}

	result := &ProcessDefinitionsResult{}
	xml.Unmarshal(b, &result)

	return result.Body.Response.ReturnValue.Values
}

func getContributorId(client *http.Client, baseUrl string, contributor string) string {
	req, err := http.NewRequest("GET", baseUrl+"/service/com.ibm.team.repository.service.internal.IAdminRestService/contributors?sortBy=name&searchTerm=%25"+contributor+"%25&pageSize=250&hideAdminGuest=false&hideUnassigned=true&hideArchivedUsers=true&pageNum=0", strings.NewReader(""))
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		panic(string(b))
	}

	type Contributor struct {
		//Name string `xml:"name"`
		ItemId string `xml:"itemId"`
	}

	type Value struct {
		Contributors []Contributor `xml:"elements"`
	}

	type ReturnValue struct {
		Value Value `xml:"value"`
	}
	type Response struct {
		Method      string      `xml:"method"`
		ReturnValue ReturnValue `xml:"returnValue"`
	}
	type Body struct {
		Response Response `xml:"response"`
	}
	type ContributorsResult struct {
		Body Body `xml:"Body"`
	}

	result := &ContributorsResult{}
	xml.Unmarshal(b, &result)

	if len(result.Body.Response.ReturnValue.Value.Contributors) != 1 {
		panic(string(b))
		panic("User " + contributor + " not found")
	}

	return result.Body.Response.ReturnValue.Value.Contributors[0].ItemId
}

func getProjectStateId(client *http.Client, baseUrl string, itemId string) string {
	req, err := http.NewRequest("GET", baseUrl+"/service/com.ibm.team.process.internal.service.web.IProcessWebUIService/projectAreaByUUIDWithLimitedMembers?processAreaItemId="+itemId+"&maxMembers=20", strings.NewReader(""))
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		panic(string(b))
	}

	type Contributor struct {
		//Name string `xml:"name"`
		ItemId string `xml:"itemId"`
	}

	type Value struct {
		StateId string `xml:"stateId"`
	}

	type ReturnValue struct {
		Value Value `xml:"value"`
	}
	type Response struct {
		Method      string      `xml:"method"`
		ReturnValue ReturnValue `xml:"returnValue"`
	}
	type Body struct {
		Response Response `xml:"response"`
	}
	type ProjectResult struct {
		Body Body `xml:"Body"`
	}

	result := &ProjectResult{}
	xml.Unmarshal(b, &result)

	if result.Body.Response.ReturnValue.Value.StateId == "" {
		panic(string(b))
		panic("Project area not found")
	}

	return result.Body.Response.ReturnValue.Value.StateId
}

func createProjectArea(client *http.Client, baseUrl string, name string, processUuid string, members string) {
	form := url.Values{}
	form.Add("itemId", "new")
	form.Add("owningApplicationKey", "JTS-Sentinel-Id")
	form.Add("name", name)
	form.Add("jsonMembers", "{}")
	form.Add("jsonAdmins", "{}")
	form.Add("processUuid", processUuid)
	form.Add("processLocale", "en-us")

	req, err := http.NewRequest("POST", baseUrl+"/service/com.ibm.team.process.internal.service.web.IProcessWebUIService/projectArea", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	cookies := client.Jar.Cookies(req.URL)
	sessionId := ""
	for _, cookie := range cookies {
		if cookie.Name == "JSESSIONID" {
			sessionId = cookie.Value
		}
	}
	req.Header.Add("X-Jazz-CSRF-Prevent", sessionId)

	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		panic(string(b))
	}

	type ReturnValue struct {
		Value string `xml:"value"`
	}
	type Response struct {
		Method      string      `xml:"method"`
		ReturnValue ReturnValue `xml:"returnValue"`
	}
	type Body struct {
		Response Response `xml:"response"`
	}
	type ProjectResult struct {
		Body Body `xml:"Body"`
	}

	result := &ProjectResult{}
	xml.Unmarshal(b, &result)

	itemId := result.Body.Response.ReturnValue.Value
	stateId := getProjectStateId(client, baseUrl, itemId)

	// Repeat with the members and roles
	form = url.Values{}
	form.Add("owningApplicationKey", "JTS-Sentinel-Id")
	form.Add("name", name)
	form.Add("jsonAdmins", "{}")
	form.Add("processUuid", processUuid)
	form.Add("processLocale", "en-us")

	form.Add("itemId", itemId)
	form.Add("stateId", stateId)

	jsonMembers := "{"

	toAdd := `"add" : [`
	roles := `"roles" : {`

	memberSlice := strings.Split(members, ",")

	for idx, memberEntry := range memberSlice {
		entry := strings.Split(memberEntry, "=")
		if len(entry) != 2 {
			panic("Invalid members parameter")
		}

		name := entry[0]
		role := entry[1]

		id := getContributorId(client, baseUrl, name)

		if idx != 0 {
			toAdd += `, `
			roles += `, `
		}

		toAdd += ` "` + id + `" `
		roles += ` "` + id + `" : ["` + role + `","default"]`
	}

	toAdd += `]`
	roles += `}`

	jsonMembers += toAdd + " , " + roles + "}"
	form.Add("jsonMembers", jsonMembers)

	req, err = http.NewRequest("POST", baseUrl+"/service/com.ibm.team.process.internal.service.web.IProcessWebUIService/projectArea", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("X-Jazz-CSRF-Prevent", sessionId)

	if err != nil {
		panic(err)
	}
	resp, err = client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	b, err = ioutil.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		panic(string(b))
	}
}

func main() {
	flag.Parse()

	client := authenticate(*repo, *userid, *password)
	if *deployMTM {
		// Deploying the MTM sample will force the process templates to be deployed
		deployMTMSample(client, strings.Replace(*repo, "/ccm", "/jts", 1), *repo)
		os.Exit(0)
	}
	if !*nodeploy {
		deployProcessTemplates(client, *repo)
	}
	chosenDefinitionId := ""
	definitions := getProcessDefinitions(client, *repo)

	if len(definitions) == 0 {
		fmt.Errorf("No process definitions found. Perhaps they need to be deployed with the '-deploy=true' option?\n")
		os.Exit(1)
	}

	if !*templates {
		for _, definition := range definitions {
			if definition.ProcessId == *processid {
				chosenDefinitionId = definition.ItemId
			}
		}

		if chosenDefinitionId == "" {
			fmt.Errorf("Could not find process definition '%v'\n", *processid)
			os.Exit(1)
		}

		createProjectArea(client, *repo, *name, chosenDefinitionId, *members)

		fmt.Printf("Project area created: %v\n", *name)
	} else {
		for _, definition := range definitions {
			fmt.Printf("NAME: %v ID: %v\n", definition.Name, definition.ProcessId)
		}
	}
}
