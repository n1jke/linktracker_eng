package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/riferrei/srclient"

	"github.com/n1jke/linktracker/config"
)

const schemaDir = "/schemas"

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("LoadConfig: %v", err)
	}

	if err := loadAvroSchema(cfg); err != nil {
		log.Fatal(err)
	}
}

func loadAvroSchema(cfg *config.AppConfig) error {
	client := srclient.CreateSchemaRegistryClient(cfg.Kafka.SchemaRegistryURL)

	schemas := []struct {
		Subject string
		File    string
	}{
		{Subject: fmt.Sprintf("%s-value", cfg.Kafka.RawUpdatesTopic), File: filepath.Join(schemaDir, "0002_raw_update.json")},
		{Subject: fmt.Sprintf("%s-value", cfg.Kafka.UpdatesTopic), File: filepath.Join(schemaDir, "0003_prepared_update.json")},
	}

	for _, s := range schemas {
		avroSchema, err := loadFromFile(s.File)
		if err != nil {
			log.Fatalf("LoadSchema %s: %v", s.File, err)
		}

		schema, err := client.CreateSchema(s.Subject, avroSchema, srclient.Avro)
		if err != nil {
			log.Fatalf("CreateSchema %s: %v", s.Subject, err)
		}

		log.Printf("Schema %s registered, ID: %d, version: %d", s.Subject, schema.ID(), schema.Version())
	}

	log.Println("All schemas registered successfully")

	return nil
}

func loadFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read schema file: %v", err)
	}

	return string(data), nil
}
