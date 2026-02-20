package encrypt

import (
	"reflect"
)

// EncryptStruct encrypts all fields with `encrypt:"true"` tag
func (s *Service) EncryptStruct(v interface{}) error {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		typeField := typ.Field(i)

		tag := typeField.Tag.Get("encrypt")
		if tag != "true" {
			continue
		}

		if field.Kind() != reflect.String || !field.CanSet() {
			continue
		}

		plaintext := field.String()
		if plaintext == "" {
			continue
		}

		encrypted, err := s.Encrypt(plaintext)
		if err != nil {
			return err
		}

		field.SetString(encrypted)
	}

	return nil
}

// DecryptStruct decrypts all fields with `encrypt:"true"` tag
func (s *Service) DecryptStruct(v interface{}) error {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		typeField := typ.Field(i)

		tag := typeField.Tag.Get("encrypt")
		if tag != "true" {
			continue
		}

		if field.Kind() != reflect.String || !field.CanSet() {
			continue
		}

		ciphertext := field.String()
		if ciphertext == "" {
			continue
		}

		decrypted, err := s.Decrypt(ciphertext)
		if err != nil {
			return err
		}

		field.SetString(decrypted)
	}

	return nil
}

// EncryptFields encrypts specific fields by name
func (s *Service) EncryptFields(v interface{}, fieldNames ...string) error {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	for _, fieldName := range fieldNames {
		field := val.FieldByName(fieldName)
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		if field.Kind() != reflect.String {
			continue
		}

		plaintext := field.String()
		if plaintext == "" {
			continue
		}

		encrypted, err := s.Encrypt(plaintext)
		if err != nil {
			return err
		}

		field.SetString(encrypted)
	}

	return nil
}

// DecryptFields decrypts specific fields by name
func (s *Service) DecryptFields(v interface{}, fieldNames ...string) error {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	for _, fieldName := range fieldNames {
		field := val.FieldByName(fieldName)
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		if field.Kind() != reflect.String {
			continue
		}

		ciphertext := field.String()
		if ciphertext == "" {
			continue
		}

		decrypted, err := s.Decrypt(ciphertext)
		if err != nil {
			return err
		}

		field.SetString(decrypted)
	}

	return nil
}
