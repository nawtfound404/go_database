package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/jcelliott/lumber"
)

const version = "1.0.0"

type (
	Logger interface {
		Fatal(string, ...interface{})
		Error(string, ...interface{})
		Warn(string, ...interface{})
		Info(string, ...interface{})
		Debug(string, ...interface{})
		Trace(string, ...interface{})
	}

	Driver struct {
		mutex   sync.Mutex
		mutexes map[string]*sync.Mutex
		dir     string
		log     Logger
	}
)
type Options struct {
	Logger
}

func New(dir string, options *Options) (*Driver, error) {
	dir = filepath.Clean(dir)

	opts := Options{}

	if options != nil {
		opts = *options
	}

	if opts.Logger == nil {
		opts.Logger = lumber.NewConsoleLogger(lumber.INFO)
	}

	driver := Driver{
		dir:     dir,
		mutexes: make(map[string]*sync.Mutex),
		log:     opts.Logger,
	}

	if _, err := os.Stat(dir); err == nil {
		opts.Logger.Debug("Using '%s' (database already exists) \n", dir)
		return &driver, nil
	}

	opts.Logger.Debug("Creating the database at '%s'\n", dir)
	return &driver, os.MkdirAll(dir, 0755)
}

func (d *Driver) Write(collection, resource string, v interface{}) error {
	if collection == "" {
		return fmt.Errorf("Missing collection - no place to save record!")
	}
	if resource == "" {
		return fmt.Errorf("Missing resource - no key to save record!")
	}

	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()

	dir := filepath.Join(d.dir, collection)
	fnlPath := filepath.Join(dir, resource+".json")
	tmpPath := fnlPath + ".tmp"

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(v, "", " \t")
	if err != nil {
		return err
	}

	b = append(b, byte('\n'))

	if err := ioutil.WriteFile(tmpPath, b, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, fnlPath)
}

func (d *Driver) Read(collection, resource string, v interface{}) error {
	if collection == "" {
		return fmt.Errorf("Missing collection - no place to save record!")
	}
	if resource == "" {
		return fmt.Errorf("Missing resource - unable to save record!")
	}

	record := filepath.Join(d.dir, collection, resource+".json")

	if _, err := stat(record); err != nil {
		return err
	}

	b, err := ioutil.ReadFile(record)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, &v)
}

func (d *Driver) ReadAll(collection string) ([]string, error) {
	if collection == "" {
		return nil, fmt.Errorf("Missing collection - unable to read record!")
	}

	dir := filepath.Join(d.dir, collection)
	records := []string{}

	if _, err := stat(dir); err != nil {
		return records, nil
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		b, err := ioutil.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return nil, err
		}
		records = append(records, string(b))
	}
	return records, nil
}

func (d *Driver) Delete(collection, resource string) error {
	path := filepath.Join(d.dir, collection, resource)
	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()

	switch fi, err := stat(path + ".json"); {
	case fi == nil, err != nil:
		return fmt.Errorf("Unable to find file or directory named %v\n", path)
	case fi.Mode().IsRegular():
		return os.Remove(path + ".json")
	}
	return nil
}

func (d *Driver) getOrCreateMutex(collection string) *sync.Mutex {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	m, ok := d.mutexes[collection]
	if !ok {
		m = &sync.Mutex{}
		d.mutexes[collection] = m
	}
	return m
}

func stat(path string) (fi os.FileInfo, err error) {
	if fi, err = os.Stat(path); os.IsNotExist(err) {
		fi, err = os.Stat(path + ".json")
	}
	return
}

type Address struct {
	City    string
	State   string
	Country string
	Pincode json.Number
}

type User struct {
	Name    string
	Age     json.Number
	Contact string
	Company string
	Address Address
}

func main() {
	dir := "./db"

	db, err := New(dir, nil)
	if err != nil {
		fmt.Println("Error", err)
		return
	}

	employees := []User{
		{"John", "25", "1234567890", "Google", Address{"Bangalore", "Karnataka", "India", "560001"}},
		{"Doe", "30", "1234567890", "Microsoft", Address{"Hyderabad", "Telangana", "India", "500001"}},
		{"Jane", "35", "1234567890", "Amazon", Address{"Mumbai", "Maharashtra", "India", "400001"}},
		{"rohan", "21", "1234567890", "Google", Address{"Bangalore", "Karnataka", "India", "560001"}},
		{"hira", "39", "1234567890", "Banu pratap bus service", Address{"Hyderabad", "Telangana", "India", "500001"}},
		{"jolly", "65", "1234567890", "Amazon", Address{"Mumbai", "Maharashtra", "India", "400001"}},
	}

	for _, value := range employees {
		err := db.Write("users", value.Name, value)
		if err != nil {
			fmt.Println("Write Error:", err)
		}
	}

	records, err := db.ReadAll("users")
	if err != nil {
		fmt.Println("ReadAll Error:", err)
		return
	}
	fmt.Println(records)

	allusers := []User{}
	for _, value := range records {
		employeeFound := User{}
		if err := json.Unmarshal([]byte(value), &employeeFound); err != nil {
			fmt.Println("Unmarshal Error:", err)
		}
		allusers = append(allusers, employeeFound)
	}
	fmt.Println(allusers)

	//if err := db.Delete("users", "John"); err != nil {
	//	fmt.Println("Delete Error:", err)
	//}
}
