package work

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/sdk"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type Settings struct {
	RepoAlias   string              `starlark:"repo_alias" env:"OCU_CFG_repo_alias"`
	RepoRemotes []string            `starlark:"repo_remotes" env:"OCU_CFG_repo_remotes"`
	State       *sdk.StorageBackend `starlark:"state_store"`
	Intent      *sdk.StorageBackend `starlark:"intent_store"`
}

func LoadSettings(be *local.BackendOutputs, globals starlark.StringDict, envVars []string) (Settings, error) {
	s := Settings{
		RepoAlias:   be.RepoAlias,
		RepoRemotes: be.RepoRemotes,
	}

	if be.Store != nil {
		s.State = &be.Store.State
		s.Intent = be.Store.Intent
	}

	err := UnmarshalFromStringDict(globals, &s)
	if err != nil {
		return s, fmt.Errorf("failed to unmarshal repo config: %w", err)
	}

	err = UnmarshalFromEnvVars(envVars, &s)
	if err != nil {
		return s, fmt.Errorf("failed to unmarshal env vars: %w", err)
	}

	return s, nil
}

func UnmarshalFromEnvVars(in []string, out any) error {
	// Parse env vars into a map
	envMap := make(map[string]string)
	for _, envVar := range in {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Get the reflect.Value of the output pointer
	outValue := reflect.ValueOf(out)
	if outValue.Kind() != reflect.Ptr {
		return fmt.Errorf("out must be a pointer, got %T", out)
	}

	// Get the underlying struct
	structValue := outValue.Elem()
	if structValue.Kind() != reflect.Struct {
		return fmt.Errorf("out must be a pointer to a struct, got pointer to %s", structValue.Kind())
	}

	// Iterate through struct fields and look for env tags
	structType := structValue.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		envTag := field.Tag.Get("env")
		if envTag == "" {
			continue
		}

		// Check if this env var exists
		envValue, exists := envMap[envTag]
		if !exists {
			continue
		}

		// Get the field value
		fieldValue := structValue.Field(i)
		if !fieldValue.CanSet() {
			return fmt.Errorf("field %s cannot be set", field.Name)
		}

		// Parse and set the value based on field type
		if err := parseEnvValue(envValue, fieldValue, field.Name); err != nil {
			return fmt.Errorf("failed to parse env var %s for field %s: %w", envTag, field.Name, err)
		}
	}

	return nil
}

func parseEnvValue(value string, fieldValue reflect.Value, fieldName string) error {
	switch fieldValue.Kind() {
	case reflect.String:
		fieldValue.SetString(value)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("cannot parse bool: %w", err)
		}
		fieldValue.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse int: %w", err)
		}
		fieldValue.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse uint: %w", err)
		}
		fieldValue.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("cannot parse float: %w", err)
		}
		fieldValue.SetFloat(f)
	case reflect.Slice, reflect.Map, reflect.Struct:
		return fmt.Errorf("unsupported type %s for env var parsing", fieldValue.Kind())
	default:
		return fmt.Errorf("unsupported type %s", fieldValue.Kind())
	}
	return nil
}

func UnmarshalFromStringDict(in starlark.StringDict, out any) error {
	for k, v := range in {
		if err := mapValueIntoAny(k, v, out); err != nil {
			return err
		}
	}
	return nil
}

func mapValueIntoAny(k string, v starlark.Value, out any) error {
	// Get the reflect.Value of the output pointer
	outValue := reflect.ValueOf(out)
	if outValue.Kind() != reflect.Ptr {
		return fmt.Errorf("out must be a pointer, got %T", out)
	}

	// Get the underlying struct
	structValue := outValue.Elem()
	if structValue.Kind() != reflect.Struct {
		return fmt.Errorf("out must be a pointer to a struct, got pointer to %s", structValue.Kind())
	}

	// Find the field with matching starlark tag
	structType := structValue.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		tag := field.Tag.Get("starlark")
		if tag == k {
			// Get the field value
			fieldValue := structValue.Field(i)
			if !fieldValue.CanSet() {
				return fmt.Errorf("field %s cannot be set", field.Name)
			}

			// If the field is a pointer and nil, allocate it
			if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
				fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
			}

			// Use the field value directly if it's a pointer, otherwise take its address
			var fieldPtr interface{}
			if fieldValue.Kind() == reflect.Ptr {
				fieldPtr = fieldValue.Interface()
			} else {
				fieldPtr = fieldValue.Addr().Interface()
			}

			return UnmarshalFromValue(v, fieldPtr)
		}
	}

	// Field not found - this might be okay, just skip it
	return nil
}

func UnmarshalFromValue(v starlark.Value, out any) error {
	// Handle None - just leave the field at its zero value
	if _, ok := v.(starlark.NoneType); ok {
		return nil
	}

	switch v := v.(type) {
	case starlark.String:
		switch out := out.(type) {
		case *string:
			*out = string(v)
		default:
			return fmt.Errorf("cannot marshal String into %T", out)
		}
	case starlark.Bool:
		switch out := out.(type) {
		case *bool:
			*out = bool(v.Truth())
		default:
			return fmt.Errorf("cannot marshal Bool into %T", out)
		}
	case starlark.Int:
		switch out := out.(type) {
		case *int:
			i, _ := v.Int64()
			*out = int(i)
		case *int64:
			i, _ := v.Int64()
			*out = i
		case *uint:
			i, _ := v.Int64()
			*out = uint(i)
		case *uint64:
			i, _ := v.Int64()
			*out = uint64(i)
		case *float64:
			i, _ := v.Int64()
			*out = float64(i)
		case *float32:
			i, _ := v.Int64()
			*out = float32(i)
		default:
			return fmt.Errorf("cannot marshal Int into %T", out)
		}
	case starlark.Float:
		switch out := out.(type) {
		case *float64:
			*out = float64(v)
		case *float32:
			*out = float32(v)
		default:
			return fmt.Errorf("cannot marshal Float into %T", out)
		}
	case *starlark.List:
		// Handle starlark.List -> Go slice
		outValue := reflect.ValueOf(out)
		if outValue.Kind() != reflect.Ptr {
			return fmt.Errorf("cannot marshal List into non-pointer %T", out)
		}
		outElem := outValue.Elem()
		if outElem.Kind() != reflect.Slice {
			return fmt.Errorf("cannot marshal List into %T", out)
		}

		// Create a new slice with the appropriate length
		sliceLen := v.Len()
		sliceType := outElem.Type()
		newSlice := reflect.MakeSlice(sliceType, sliceLen, sliceLen)

		// Unmarshal each element
		iter := v.Iterate()
		defer iter.Done()
		var idx int
		var item starlark.Value
		for iter.Next(&item) {
			elemPtr := newSlice.Index(idx).Addr().Interface()
			if err := UnmarshalFromValue(item, elemPtr); err != nil {
				return fmt.Errorf("cannot unmarshal list element %d: %w", idx, err)
			}
			idx++
		}

		outElem.Set(newSlice)
	case *starlark.Dict:
		outValue := reflect.ValueOf(out)
		if outValue.Kind() != reflect.Ptr {
			return fmt.Errorf("cannot marshal Dict into non-pointer %T", out)
		}
		outElem := outValue.Elem()

		if outElem.Kind() == reflect.Map {
			// Handle starlark.Dict -> Go map
			// Check that the map key type is string
			mapType := outElem.Type()
			if mapType.Key().Kind() != reflect.String {
				return fmt.Errorf("cannot marshal Dict into map with non-string keys: %T", out)
			}

			// Create a new map
			newMap := reflect.MakeMap(mapType)

			// Unmarshal each key-value pair
			for _, item := range v.Items() {
				key, ok := item[0].(starlark.String)
				if !ok {
					return fmt.Errorf("dict key is not a string: %T", item[0])
				}
				keyStr := string(key)

				// Create a new value of the map's value type
				valueType := mapType.Elem()
				valuePtr := reflect.New(valueType)

				if err := UnmarshalFromValue(item[1], valuePtr.Interface()); err != nil {
					return fmt.Errorf("cannot unmarshal dict value for key %q: %w", keyStr, err)
				}

				newMap.SetMapIndex(reflect.ValueOf(keyStr), valuePtr.Elem())
			}

			outElem.Set(newMap)
		} else if outElem.Kind() == reflect.Struct {
			// Handle starlark.Dict -> Go struct
			// Unmarshal each key-value pair into struct fields
			for _, item := range v.Items() {
				key, ok := item[0].(starlark.String)
				if !ok {
					return fmt.Errorf("dict key is not a string: %T", item[0])
				}
				keyStr := string(key)

				// Find the corresponding Go struct field by json tag
				structType := outElem.Type()
				for i := 0; i < structType.NumField(); i++ {
					field := structType.Field(i)
					tag := field.Tag.Get("json")
					// Remove omitempty and other options from json tag
					if idx := strings.Index(tag, ","); idx != -1 {
						tag = tag[:idx]
					}
					if tag == keyStr {
						fieldValue := outElem.Field(i)
						if !fieldValue.CanSet() {
							return fmt.Errorf("field %s cannot be set", field.Name)
						}

						// If the field is a pointer and nil, allocate it
						if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
							fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
						}

						// Use the field value directly if it's a pointer, otherwise take its address
						var fieldPtr interface{}
						if fieldValue.Kind() == reflect.Ptr {
							fieldPtr = fieldValue.Interface()
						} else {
							fieldPtr = fieldValue.Addr().Interface()
						}

						if err := UnmarshalFromValue(item[1], fieldPtr); err != nil {
							return fmt.Errorf("cannot unmarshal dict field %s: %w", keyStr, err)
						}
						break
					}
				}
			}
		} else {
			return fmt.Errorf("cannot marshal Dict into %T", out)
		}
	case *starlarkstruct.Struct:
		// Handle starlark.Struct -> Go struct pointer
		outValue := reflect.ValueOf(out)
		if outValue.Kind() != reflect.Ptr {
			return fmt.Errorf("cannot marshal Struct into non-pointer %T", out)
		}
		outElem := outValue.Elem()
		if outElem.Kind() != reflect.Struct {
			return fmt.Errorf("cannot marshal Struct into %T", out)
		}

		// Iterate through the Starlark struct's attributes
		attrNames := v.AttrNames()
		for _, attrName := range attrNames {
			attrValue, err := v.Attr(attrName)
			if err != nil {
				return fmt.Errorf("failed to get attribute %s: %w", attrName, err)
			}

			// Find the corresponding Go struct field by starlark or json tag
			structType := outElem.Type()
			for i := 0; i < structType.NumField(); i++ {
				field := structType.Field(i)
				// Check starlark tag first, then json tag
				tag := field.Tag.Get("starlark")
				if tag == "" {
					tag = field.Tag.Get("json")
					// Remove omitempty and other options from json tag
					if idx := strings.Index(tag, ","); idx != -1 {
						tag = tag[:idx]
					}
				}
				if tag == attrName {
					fieldValue := outElem.Field(i)
					if !fieldValue.CanSet() {
						return fmt.Errorf("field %s cannot be set", field.Name)
					}

					// If the field is a pointer and nil, allocate it
					if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
						fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
					}

					// Use the field value directly if it's a pointer, otherwise take its address
					var fieldPtr interface{}
					if fieldValue.Kind() == reflect.Ptr {
						fieldPtr = fieldValue.Interface()
					} else {
						fieldPtr = fieldValue.Addr().Interface()
					}

					if err := UnmarshalFromValue(attrValue, fieldPtr); err != nil {
						return fmt.Errorf("cannot unmarshal struct field %s: %w", attrName, err)
					}
					break
				}
			}
		}
	default:
		return fmt.Errorf("unhandled type: %T", v)
	}

	return nil
}
