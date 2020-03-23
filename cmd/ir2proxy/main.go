package main

import (
	"bytes"
	"html/template"
	"net/http"
	"strings"

	"github.com/ghodss/yaml"

	"github.com/projectcontour/ir2proxy/internal/k8sdecoder"
	"github.com/projectcontour/ir2proxy/internal/translator"
	"github.com/projectcontour/ir2proxy/internal/validate"
	"github.com/sirupsen/logrus"
)

func commentedWarnings(warnings []string) string {
	for index, warning := range warnings {
		warnings[index] = "# " + strings.ReplaceAll(warning, ". ", ".\n# ")
	}
	return strings.Join(warnings, "\n")

}

func processInput(w http.ResponseWriter, req *http.Request) {

	log := logrus.StandardLogger()
	var output string
	var outputWarnings string
	var outputYAML []byte

	tmpl := template.Must(template.ParseFiles("forms.html"))

	if req.Method != http.MethodPost {
		tmpl.Execute(w, nil)
		return
	}

	input := req.FormValue("input")
	log.Println("##### FROM form: " + input)

	ir, err := k8sdecoder.DecodeIngressRoute([]byte(input))
	if err != nil {
		log.Error(err)
		log.Printf("Error from k8sdecoder.DecodeIngressRoute(input)")
		output = err.Error()
		if err := tmpl.Execute(
			w,
			map[string]interface{}{
				"input":  input,
				"output": output,
			}); err != nil {
			log.Println("Error executing", err)
		}
		return
	}

	validationErrors := validate.CheckIngressRoute(ir)
	if len(validationErrors) > 0 {
		for _, validationError := range validationErrors {
			log.Error(validationError)
			log.Printf("Error from validate.CheckIngressRoute(ir)")
			output += err.Error()
			if err := tmpl.Execute(
				w,
				map[string]interface{}{
					"input":  input,
					"output": output,
				}); err != nil {
				log.Println("Error executing", err)
			}
		}
		return
	}

	hp, warnings, err := translator.IngressRouteToHTTPProxy(ir)
	if err != nil {
		log.Error(err)
		log.Printf("Error from translator.IngressRouteToHTTPProxy(ir)")
		output = err.Error()
		if err := tmpl.Execute(
			w,
			map[string]interface{}{
				"input":  input,
				"output": output,
			}); err != nil {
			log.Println("Error executing", err)
		}
		return
	}

	for _, warning := range warnings {
		log.Warn(warning)
	}

	outputYAML, err = yaml.Marshal(hp)
	if err != nil {
		log.Warn(err)
		log.Printf("Error from yaml.Marshal(hp)")
		output = err.Error()
		if err := tmpl.Execute(
			w,
			map[string]interface{}{
				"input":  input,
				"output": output,
			}); err != nil {
			log.Println("Error executing", err)
		}
		return
	}

	// The Kubernetes standard header field `currentTimestamp` serializes weirdly,
	// so filter it out.
	// See https://github.com/projectcontour/ir2proxy/issues/8 for more explanation here.
	outputYAML = bytes.ReplaceAll(outputYAML, []byte("  creationTimestamp: null\n"), []byte(""))
	outputYAML = bytes.ReplaceAll(outputYAML, []byte("status: {}"), []byte(""))
	outputWarnings = commentedWarnings(warnings)
	// fmt.Printf("---\n%s%s", outputWarnings, outputYAML)
	output = string(outputWarnings) + string(outputYAML)

	log.Println("### PRE template: " + output)

	if err := tmpl.Execute(
		w,
		map[string]interface{}{
			"input":  input,
			"output": output,
		}); err != nil {
		log.Println("Error executing", err)
	}

	log.Println("### POST template: " + output)

}

func main() {

	http.HandleFunc("/", processInput)
	http.ListenAndServe(":8080", nil)

}
