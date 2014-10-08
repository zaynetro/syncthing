// Copyright (C) 2014 Jakob Borg and Contributors (see the CONTRIBUTORS file).
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for
// more details.
//
// You should have received a copy of the GNU General Public License along
// with this program. If not, see <http://www.gnu.org/licenses/>.

// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
)

type stat struct {
	Translated   int `json:"translated_entities"`
	Untranslated int `json:"untranslated_entities"`
}

type translation struct {
	Content string
}

func main() {
	log.SetFlags(log.Lshortfile)

	if u, p := userPass(); u == "" || p == "" {
		log.Fatal("Need environment variables TRANSIFEX_USER and TRANSIFEX_PASS")
	}

	curValidLangs := map[string]bool{}
	for _, lang := range loadValidLangs() {
		curValidLangs[lang] = true
	}
	log.Println(curValidLangs)

	resp := req("https://www.transifex.com/api/2/project/syncthing/resource/gui/stats")

	var stats map[string]stat
	err := json.NewDecoder(resp.Body).Decode(&stats)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	var langs []string
	for code, stat := range stats {
		code = strings.Replace(code, "_", "-", 1)
		pct := 100 * stat.Translated / (stat.Translated + stat.Untranslated)
		if pct < 75 || !curValidLangs[code] && pct < 95 {
			log.Printf("Skipping language %q (too low completion ratio %d%%)", code, pct)
			os.Remove("lang-" + code + ".json")
			continue
		}

		langs = append(langs, code)
		if code == "en" {
			continue
		}

		log.Printf("Updating language %q", code)

		resp := req("https://www.transifex.com/api/2/project/syncthing/resource/gui/translation/" + code)
		var t translation
		err := json.NewDecoder(resp.Body).Decode(&t)
		if err != nil {
			log.Fatal(err)
		}
		resp.Body.Close()

		fd, err := os.Create("lang-" + code + ".json")
		if err != nil {
			log.Fatal(err)
		}
		fd.WriteString(t.Content)
		fd.Close()
	}

	saveValidLangs(langs)
}

func saveValidLangs(langs []string) {
	sort.Strings(langs)
	fd, err := os.Create("valid-langs.js")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprint(fd, "var validLangs = ")
	json.NewEncoder(fd).Encode(langs)
	fd.Close()
}

func userPass() (string, string) {
	user := os.Getenv("TRANSIFEX_USER")
	pass := os.Getenv("TRANSIFEX_PASS")
	return user, pass
}

func req(url string) *http.Response {
	user, pass := userPass()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.SetBasicAuth(user, pass)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	return resp
}

func loadValidLangs() []string {
	fd, err := os.Open("valid-langs.js")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()
	bs, err := ioutil.ReadAll(fd)
	if err != nil {
		log.Fatal(err)
	}

	var langs []string
	exp := regexp.MustCompile(`\[([a-zA-Z",-]+)\]`)
	if matches := exp.FindSubmatch(bs); len(matches) == 2 {
		langs = strings.Split(string(matches[1]), ",")
		for i := range langs {
			// Remove quotes
			langs[i] = langs[i][1 : len(langs[i])-1]
		}
	}

	return langs
}