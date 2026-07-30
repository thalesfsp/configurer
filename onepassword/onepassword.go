package onepassword

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/validation"
)

//////
// Vars, consts, and types.
//////

const (
	// Name of the provider.
	Name = "onepassword"

	concealedFieldType = "CONCEALED"
	secureNoteCategory = "SECURE_NOTE"
)

var connectUUIDPattern = regexp.MustCompile(`^[a-z0-9]{26}$`)

// Config contains 1Password Connect authentication and item settings.
type Config struct {
	Host  string `json:"host"  validate:"required"`
	Token string `json:"-"     validate:"required"`
	Vault string `json:"vault" validate:"required"`
	Item  string `json:"item"  validate:"required"`
}

// OnePassword provider definition.
type OnePassword struct {
	*provider.Provider `json:"-" validate:"required"`

	Configuration *Config      `json:"-" validate:"required"`
	client        *http.Client `json:"-" validate:"required"`
}

type vaultSummary struct {
	ID string `json:"id"`
}

type itemSummary struct {
	ID string `json:"id"`
}

type itemResponse struct {
	Fields []itemField `json:"fields"`
}

type itemField struct {
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

//////
// IProvider implementation.
//////

// Load retrieves an item from 1Password Connect and exports its fields to the
// environment.
func (o *OnePassword) Load(ctx context.Context, opts ...option.LoadKeyFunc) (map[string]string, error) {
	vaultID, err := o.resolveVault(ctx)
	if err != nil {
		return nil, err
	}

	itemID, found, err := o.resolveItem(ctx, vaultID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, customerror.NewMissingError(
			fmt.Sprintf("1Password item %q", o.Configuration.Item),
		)
	}

	var item itemResponse
	if _, err := o.doJSON(
		ctx,
		http.MethodGet,
		fmt.Sprintf("/v1/vaults/%s/items/%s", url.PathEscape(vaultID), url.PathEscape(itemID)),
		nil,
		nil,
		&item,
	); err != nil {
		return nil, customerror.NewFailedToError("load 1Password item", customerror.WithError(err))
	}

	finalValues := make(map[string]string)

	for _, field := range item.Fields {
		key := field.Label
		if key == "" {
			key = field.ID
		}
		if key == "" || field.Value == "" {
			continue
		}

		for _, opt := range opts {
			key = opt(key)
		}

		finalValue, err := provider.ExportToEnvVar(o, key, field.Value)
		if err != nil {
			return nil, err
		}

		finalValues[key] = finalValue
	}

	return finalValues, nil
}

// Write upserts fields on an item through 1Password Connect.
func (o *OnePassword) Write(ctx context.Context, values map[string]interface{}, opts ...option.WriteFunc) error {
	if values == nil {
		return customerror.NewRequiredError("values")
	}

	var options option.Write
	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	vaultID, err := o.resolveVault(ctx)
	if err != nil {
		return err
	}

	itemID, found, err := o.resolveItem(ctx, vaultID)
	if err != nil {
		return err
	}

	var item map[string]interface{}
	if found {
		statusCode, err := o.doJSON(
			ctx,
			http.MethodGet,
			fmt.Sprintf("/v1/vaults/%s/items/%s", url.PathEscape(vaultID), url.PathEscape(itemID)),
			nil,
			nil,
			&item,
		)
		if err != nil && statusCode != http.StatusNotFound {
			return customerror.NewFailedToError("load 1Password item for update", customerror.WithError(err))
		}
		if statusCode == http.StatusNotFound {
			found = false
		}
	}

	if !found {
		item = map[string]interface{}{
			"title":    o.Configuration.Item,
			"category": secureNoteCategory,
			"vault": map[string]interface{}{
				"id": vaultID,
			},
			"fields": fieldsFromValues(values),
		}

		if _, err := o.doJSON(
			ctx,
			http.MethodPost,
			fmt.Sprintf("/v1/vaults/%s/items", url.PathEscape(vaultID)),
			nil,
			item,
			nil,
		); err != nil {
			return customerror.NewFailedToError("create 1Password item", customerror.WithError(err))
		}

		return nil
	}

	if err := mergeItemFields(item, values); err != nil {
		return err
	}

	if _, err := o.doJSON(
		ctx,
		http.MethodPut,
		fmt.Sprintf("/v1/vaults/%s/items/%s", url.PathEscape(vaultID), url.PathEscape(itemID)),
		nil,
		item,
		nil,
	); err != nil {
		return customerror.NewFailedToError("update 1Password item", customerror.WithError(err))
	}

	return nil
}

//////
// Resolution and API helpers.
//////

func (o *OnePassword) resolveVault(ctx context.Context) (string, error) {
	if isConnectUUID(o.Configuration.Vault) {
		return o.Configuration.Vault, nil
	}

	var vaults []vaultSummary
	if _, err := o.doJSON(
		ctx,
		http.MethodGet,
		"/v1/vaults",
		url.Values{"filter": {fmt.Sprintf("name eq %s", strconv.Quote(o.Configuration.Vault))}},
		nil,
		&vaults,
	); err != nil {
		return "", customerror.NewFailedToError("resolve 1Password vault", customerror.WithError(err))
	}

	switch len(vaults) {
	case 0:
		return "", customerror.NewMissingError(
			fmt.Sprintf("1Password vault %q", o.Configuration.Vault),
		)
	case 1:
		return vaults[0].ID, nil
	default:
		return "", customerror.NewInvalidError(
			fmt.Sprintf("1Password vault %q is ambiguous", o.Configuration.Vault),
		)
	}
}

func (o *OnePassword) resolveItem(ctx context.Context, vaultID string) (string, bool, error) {
	if isConnectUUID(o.Configuration.Item) {
		return o.Configuration.Item, true, nil
	}

	var items []itemSummary
	if _, err := o.doJSON(
		ctx,
		http.MethodGet,
		fmt.Sprintf("/v1/vaults/%s/items", url.PathEscape(vaultID)),
		url.Values{"filter": {fmt.Sprintf("title eq %s", strconv.Quote(o.Configuration.Item))}},
		nil,
		&items,
	); err != nil {
		return "", false, customerror.NewFailedToError("resolve 1Password item", customerror.WithError(err))
	}

	switch len(items) {
	case 0:
		return "", false, nil
	case 1:
		return items[0].ID, true, nil
	default:
		return "", false, customerror.NewInvalidError(
			fmt.Sprintf("1Password item %q is ambiguous", o.Configuration.Item),
		)
	}
}

func (o *OnePassword) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	requestBody, responseBody interface{},
) (int, error) {
	endpoint, err := url.Parse(strings.TrimRight(o.Configuration.Host, "/") + path)
	if err != nil {
		return 0, customerror.NewFailedToError("parse 1Password Connect URL", customerror.WithError(err))
	}
	if query != nil {
		endpoint.RawQuery = query.Encode()
	}

	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return 0, customerror.NewFailedToError("marshal 1Password request", customerror.WithError(err))
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return 0, customerror.NewFailedToError("build 1Password request", customerror.WithError(err))
	}

	request.Header.Set("Authorization", "Bearer "+o.Configuration.Token)
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := o.client.Do(request)
	if err != nil {
		return 0, customerror.NewFailedToError("call 1Password Connect API", customerror.WithError(err))
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		responseData, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))

		return response.StatusCode, customerror.NewFailedToError(
			fmt.Sprintf("call 1Password Connect API: HTTP %d", response.StatusCode),
			customerror.WithStatusCode(response.StatusCode),
			customerror.WithField("responseBody", string(responseData)),
		)
	}

	if responseBody != nil {
		if err := json.NewDecoder(response.Body).Decode(responseBody); err != nil {
			return response.StatusCode, customerror.NewFailedToError(
				"decode 1Password response",
				customerror.WithError(err),
			)
		}
	}

	return response.StatusCode, nil
}

func isConnectUUID(value string) bool {
	return connectUUIDPattern.MatchString(value)
}

func fieldsFromValues(values map[string]interface{}) []itemField {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fields := make([]itemField, 0, len(keys))
	for _, key := range keys {
		fields = append(fields, itemField{
			Label: key,
			Type:  concealedFieldType,
			Value: fmt.Sprint(values[key]),
		})
	}

	return fields
}

func mergeItemFields(item map[string]interface{}, values map[string]interface{}) error {
	rawFields, exists := item["fields"]
	if !exists {
		item["fields"] = fieldsFromValues(values)

		return nil
	}

	fields, ok := rawFields.([]interface{})
	if !ok {
		return customerror.NewInvalidError("1Password item fields must be an array")
	}

	remaining := make(map[string]interface{}, len(values))
	for key, value := range values {
		remaining[key] = value
	}

	for _, rawField := range fields {
		field, ok := rawField.(map[string]interface{})
		if !ok {
			return customerror.NewInvalidError("1Password item field must be an object")
		}

		key, _ := field["label"].(string)
		if key == "" {
			key, _ = field["id"].(string)
		}

		value, replace := remaining[key]
		if !replace {
			continue
		}

		field["type"] = concealedFieldType
		field["value"] = fmt.Sprint(value)
		delete(remaining, key)
	}

	newFields := fieldsFromValues(remaining)
	for _, field := range newFields {
		fields = append(fields, map[string]interface{}{
			"label": field.Label,
			"type":  field.Type,
			"value": field.Value,
		})
	}
	item["fields"] = fields

	return nil
}

//////
// Factory.
//////

// New creates a 1Password Connect provider.
func New(override, rawValue bool, config *Config) (provider.IProvider, error) {
	if config == nil {
		return nil, customerror.NewRequiredError("config")
	}

	if err := validation.Validate(config); err != nil {
		return nil, err
	}

	baseProvider, err := provider.New(Name, override, rawValue)
	if err != nil {
		return nil, err
	}

	onePassword := &OnePassword{
		Provider:      baseProvider,
		Configuration: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if err := validation.Validate(onePassword); err != nil {
		return nil, err
	}

	return onePassword, nil
}
