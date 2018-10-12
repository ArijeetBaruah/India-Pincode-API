package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/bamzi/jobrunner"
	"github.com/go-zoo/bone"
)

type ResponseJSON struct {
	Record []*ResponseJSONRecord `json:"records"`
}

type ResponseJSONRecord struct {
	Pincode      string `json:"pincode"`
	DivisionName string `json:"divisionname"`
	RegionName   string `json:"regionname"`
	CircleName   string `json:"circlename"`
	Taluk        string `json:"taluk"`
	DistrictName string `json:"districtname"`
	StateName    string `json:"statename"`
	Long         string `json:"longitude"`
	Lang         string `json:"latitude"`
}

type PincodeOutput struct {
	State map[string][]*ResponseJSONRecord `json:"State"`
}

type GeneratePin struct {
}

func main() {
	RunCron()

	mux := bone.New()

	mux.Get("/", http.HandlerFunc(GetPin))
	mux.Get("/cron", http.HandlerFunc(GetCronPage))

	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Panic(err)
	}
}

func GetPin(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadFile("./pincode.json")
	if err != nil {
		log.Panic(err)
	}

	w.Header().Set("Content-type", "application/json")
	w.Write(data)
}

func GetCronPage(w http.ResponseWriter, r *http.Request) {
	pageData := jobrunner.StatusPage()

	tpl, err := template.ParseFiles("./views/Status.html")
	if err != nil {
		log.Panic(err)
	}

	if err := tpl.Execute(w, pageData); err != nil {
		log.Panic(err)
	}
}

func (gp GeneratePin) Run() {
	log.Println("Starting")

	for i := 0; i <= 154797; i += 10 {
		log.Printf("\x0c progress %d/%d \n", i, 154797)
		pincodes := &PincodeOutput{}
		pincodes.State = make(map[string][]*ResponseJSONRecord)
		records := gp.fetch(i)

		var wg sync.WaitGroup
		for _, record := range records {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pincodes.State[record.CircleName] = append(pincodes.State[record.CircleName], record)
			}()
			wg.Wait()
		}

		data, err := json.Marshal(pincodes)
		if err != nil {
			log.Panic(err)
		}

		os.Mkdir("tmp", 0777)

		if err := ioutil.WriteFile(fmt.Sprintf("tmp/pincodes_%d.json", i), data, 0777); err != nil {
			log.Panic(err)
		}

		if i%200 == 0 && i != 0 {
			gp.compileData()
		}
	}

	gp.compileData()

	dst, err := os.Create("./pincode.json")
	if err != nil {
		log.Panic(err)
	}

	in, err := os.Open("tmp/c_pincodes.json")
	if err != nil {
		log.Panic(err)
	}

	if _, err := io.Copy(dst, in); err != nil {
		log.Panic(err)
	}

	os.RemoveAll("tmp/")

	fmt.Println("done")

}

func (gp GeneratePin) compileData() {
	compiledJSON := &PincodeOutput{}
	compiledJSON.State = make(map[string][]*ResponseJSONRecord)
	files, err := ioutil.ReadDir("tmp/")
	if err != nil {
		log.Panic(err)
	}

	for _, f := range files {
		rawJSON := &PincodeOutput{}
		fileData, err := ioutil.ReadFile("tmp/" + f.Name())
		if err != nil {
			log.Panic(err)
		}
		if err := json.Unmarshal(fileData, &rawJSON); err != nil {
			log.Panic(err)
		}
		for key, state := range rawJSON.State {
			oldData := compiledJSON.State[key]
			oldData = append(oldData, state...)
			compiledJSON.State[key] = oldData
		}

		os.Remove("tmp/" + f.Name())
		if err != nil {
			log.Panic(err)
		}

		data, err := json.Marshal(compiledJSON)

		if err := ioutil.WriteFile("tmp/c_pincodes.json", data, 0777); err != nil {
			log.Panic(err)
		}
	}
}

func (gp GeneratePin) fetch(offset int) []*ResponseJSONRecord {
	// start := time.Now()
	url := fmt.Sprintf(`https://api.data.gov.in/resource/04cbe4b1-2f2b-4c39-a1d5-1c2e28bc0e32?api-key=579b464db66ec23bdd000001cdd3946e44ce4aad7209ff7b23ac571b&format=json&offset=%d&limit=10`, offset)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Panic(err)
	}
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Panic(err)
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Panic(err)
	}

	resJSON := &ResponseJSON{}

	if err := json.Unmarshal(data, &resJSON); err != nil {
		log.Panic(err)
	}

	return resJSON.Record
}

func RunCron() {
	jobrunner.Start()
	jobrunner.Now(GeneratePin{})
	jobrunner.Schedule("@daily", GeneratePin{})
}
