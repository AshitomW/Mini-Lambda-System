package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)


type Function struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Image string `json:"image"`
	CreatedAt time.Time `json:"created_at"`
}


var(
	functionsMutex sync.Mutex
	functions = map[string]Function{}
	dataDirectory = "function"
)

func init(){
	os.MkdirAll(dataDirectory,0755);
	loadFunctions();
}


func loadFunctions(){
	file := filepath.Join(dataDirectory, "functions.json")
	data, err := os.ReadFile(file)
	if err == nil {
		_ = json.Unmarshal(data,&functions)
	}
}

func saveFunctions(){
	file := filepath.Join(dataDirectory,"functions.json")
	data, _ := json.MarshalIndent(functions,""," ")
	_ = os.WriteFile(file,data,0644);
}


func RegisterFunction(name, image string) Function {
	functionsMutex.Lock();
	defer functionsMutex.Unlock()

	f := Function{
		ID: uuid.NewString(),
		Name: name, 
		Image: image,
		CreatedAt: time.Now(),
	}

	functions[f.ID] = f

	saveFunctions()
	return f
}


func GetFunction(id string) (Function,error){
	functionsMutex.Lock();
	defer functionsMutex.Unlock();

	f, ok := functions[id]

	if !ok {
		return Function{},errors.New("Function Not Found")
	}

	return f, nil
}



func ListFunction() []Function{
	functionsMutex.Lock()
	defer functionsMutex.Unlock()

	out := []Function{}
	for _ , f := range functions{
		out = append(out, f)
	}
	return out
}


func ListAvailableDockerImages() ([]string,error) {
	images, err := ListDockerImages()
	if err != nil{
		return nil, err
	}
	return images, nil
}