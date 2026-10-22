package commander

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"fyne.io/fyne/v2"
)

// settingsFile keeps Fyne's preference API and makes JSON authoritative at startup.
// ponytail: manual edits load on restart; no live reload or multi-process merging.
type settingsFile struct {
	prefs      fyne.Preferences
	path       string
	extra      map[string]json.RawMessage
	stateExtra map[string]json.RawMessage
	dirty      chan struct{}
	stop       chan struct{}
	done       chan struct{}
	onError    func(error)
	once       sync.Once
	err        error
}

func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dualcommander", "settings.json"), nil
}

func settingsValues(p fyne.Preferences) map[string]any {
	return map[string]any{
		prefLeft: p.String(prefLeft), prefRight: p.String(prefRight),
		prefLeftBrief: p.Bool(prefLeftBrief), prefRightBrief: p.Bool(prefRightBrief),
		prefLeftSort: p.String(prefLeftSort), prefRightSort: p.String(prefRightSort),
		prefActive: p.StringWithFallback(prefActive, "left"),
		prefTheme:  p.StringWithFallback(prefTheme, "system"),
		prefHidden: p.Bool(prefHidden), prefDensity: p.StringWithFallback(prefDensity, "standard"),
		prefFontSize:       p.StringWithFallback(prefFontSize, "medium"),
		prefTrash:          p.BoolWithFallback(prefTrash, true),
		prefBrowseArchives: p.BoolWithFallback(prefBrowseArchives, true),
		prefRevealCmd:      p.String(prefRevealCmd), prefTerminalCmd: p.String(prefTerminalCmd),
		prefEditorCmd:       p.String(prefEditorCmd),
		prefCompareFilesCmd: p.String(prefCompareFilesCmd), prefCompareFoldersCmd: p.String(prefCompareFoldersCmd),
		prefW: p.FloatWithFallback(prefW, 1100), prefH: p.FloatWithFallback(prefH, 700),
		prefFavourites: slices.Clone(p.StringListWithFallback(prefFavourites, []string{})),
	}
}

func isStateKey(key string) bool {
	switch key {
	case prefLeft, prefRight, prefLeftSort, prefRightSort, prefActive, prefW, prefH:
		return true
	}
	return false
}

var stateFields = map[string][]string{
	prefActive: {"pane", "active"},
	prefLeft:   {"pane", "left", "path"}, prefLeftSort: {"pane", "left", "sort"},
	prefRight: {"pane", "right", "path"}, prefRightSort: {"pane", "right", "sort"},
	prefW: {"window", "width"}, prefH: {"window", "height"},
}

var settingsFields = map[string][]string{
	prefTheme: {"appearance", "theme"}, prefDensity: {"appearance", "density"},
	prefFontSize:  {"appearance", "fontSize"},
	prefLeftBrief: {"views", "left", "brief"}, prefRightBrief: {"views", "right", "brief"},
	prefHidden:            {"views", "showHidden"},
	prefTrash:             {"deletion", "moveToTrash"},
	prefBrowseArchives:    {"archives", "browse"},
	prefRevealCmd:         {"externalApplications", "revealCommand"},
	prefTerminalCmd:       {"externalApplications", "terminalCommand"},
	prefEditorCmd:         {"externalApplications", "editorCommand"},
	prefCompareFilesCmd:   {"externalApplications", "compareFilesCommand"},
	prefCompareFoldersCmd: {"externalApplications", "compareFoldersCommand"},
}

// stateObject validates objects before startup can overwrite any source file.
func stateObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	object := map[string]json.RawMessage{}
	if raw != nil {
		if err := json.Unmarshal(raw, &object); err != nil || object == nil {
			return nil, fmt.Errorf("invalid JSON group: expected an object")
		}
	}
	return object, nil
}

func flattenState(state map[string]json.RawMessage) error {
	return flattenFields(state, stateFields)
}

func flattenFields(state map[string]json.RawMessage, fields map[string][]string) error {
	for key, path := range fields {
		object := state
		for _, name := range path[:len(path)-1] {
			var err error
			object, err = stateObject(object[name])
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
		if raw, ok := object[path[len(path)-1]]; ok {
			state[key] = raw
		}
	}
	return nil
}

func nestState(state map[string]json.RawMessage, path []string, value json.RawMessage) error {
	if len(path) == 1 {
		state[path[0]] = value
		return nil
	}
	child, err := stateObject(state[path[0]])
	if err != nil {
		return err
	}
	if err := nestState(child, path[1:], value); err != nil {
		return err
	}
	state[path[0]], err = json.Marshal(child)
	return err
}

func openSettingsFile(p fyne.Preferences, path string, onError func(error)) (*settingsFile, error) {
	s := &settingsFile{prefs: p, path: path, extra: map[string]json.RawMessage{},
		dirty: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}), onError: onError}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	favouritesPath := filepath.Join(filepath.Dir(path), "favourites.json")
	favouritesData, favouritesErr := os.ReadFile(favouritesPath)
	if favouritesErr != nil && !os.IsNotExist(favouritesErr) {
		return nil, favouritesErr
	}
	var favourites []string
	if favouritesErr == nil {
		if err := json.Unmarshal(favouritesData, &favourites); err != nil || favourites == nil {
			return nil, fmt.Errorf("invalid favourites JSON in %s: expected an array of paths (%v)", favouritesPath, err)
		}
	}
	if err == nil {
		if err := json.Unmarshal(data, &s.extra); err != nil || s.extra == nil {
			return nil, fmt.Errorf("invalid settings JSON in %s: expected an object (%v)", path, err)
		}
		if err := flattenFields(s.extra, settingsFields); err != nil {
			return nil, err
		}
	}
	statePath := filepath.Join(filepath.Dir(path), "state.json")
	stateData, stateErr := os.ReadFile(statePath)
	if stateErr != nil && !os.IsNotExist(stateErr) {
		return nil, stateErr
	}
	if stateErr == nil {
		if err := json.Unmarshal(stateData, &s.stateExtra); err != nil || s.stateExtra == nil {
			return nil, fmt.Errorf("invalid state JSON in %s: expected an object (%v)", statePath, err)
		}
		if err := flattenState(s.stateExtra); err != nil {
			return nil, err
		}
		for key := range settingsValues(p) {
			if isStateKey(key) {
				delete(s.extra, key)
			}
		}
		for key, raw := range s.stateExtra {
			if isStateKey(key) {
				s.extra[key] = raw
			}
		}
	}
	if favouritesErr == nil {
		delete(s.extra, prefFavourites) // Separate file is authoritative, even over stale legacy types.
	}
	if err == nil || stateErr == nil {
		// Validate every value before changing any preferences.
		values := make(map[string]any)
		for key, fallback := range settingsValues(p) {
			raw, exists := s.extra[key]
			if !exists {
				continue
			}
			var target any
			switch fallback.(type) {
			case string:
				target = new(string)
			case bool:
				target = new(bool)
			case float64:
				target = new(float64)
			case []string:
				target = new([]string)
			}
			if string(raw) == "null" {
				return nil, fmt.Errorf("invalid setting %q: null is not supported", key)
			}
			if err := json.Unmarshal(raw, target); err != nil {
				return nil, fmt.Errorf("invalid setting %q: %w", key, err)
			}
			if key == prefActive && *target.(*string) != "left" && *target.(*string) != "right" {
				return nil, fmt.Errorf("invalid active pane: expected left or right")
			}
			if v, ok := target.(*float64); ok && (*v <= 0 || *v > 100000) {
				return nil, fmt.Errorf("invalid setting %q: window size must be between 0 and 100000", key)
			}
			values[key] = target
		}
		for key := range settingsValues(p) {
			if err == nil || isStateKey(key) {
				p.RemoveValue(key) // Missing JSON keys use defaults, not stale legacy values.
			}
		}
		for key, value := range values {
			switch v := value.(type) {
			case *string:
				p.SetString(key, *v)
			case *bool:
				p.SetBool(key, *v)
			case *float64:
				p.SetFloat(key, *v)
			case *[]string:
				p.SetStringList(key, *v)
			}
		}
	}
	if favouritesErr == nil {
		p.SetStringList(prefFavourites, favourites)
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	p.AddChangeListener(func() {
		select {
		case s.dirty <- struct{}{}:
		default:
		}
	})
	go func() {
		defer close(s.done)
		for {
			select {
			case <-s.dirty:
				if err := s.save(); err != nil && s.onError != nil {
					s.onError(err)
				}
			case <-s.stop:
				s.err = s.save()
				return
			}
		}
	}()
	return s, nil
}

func (s *settingsFile) close() error {
	s.once.Do(func() { close(s.stop) })
	<-s.done
	return s.err
}

func (s *settingsFile) save() error {
	values := make(map[string]json.RawMessage, len(s.extra))
	state := make(map[string]json.RawMessage, len(s.stateExtra))
	for key, value := range s.stateExtra {
		state[key] = value
	}
	for key, value := range s.extra {
		values[key] = value
	}
	for key, value := range settingsValues(s.prefs) {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		values[key] = raw
	}
	for key, value := range values {
		if isStateKey(key) {
			raw, err := json.Marshal(value)
			if err != nil {
				return err
			}
			if err := nestState(state, stateFields[key], raw); err != nil {
				return err
			}
			delete(values, key)
		}
	}
	for key := range stateFields {
		delete(state, key)
	}
	favourites := values[prefFavourites]
	delete(values, prefFavourites)
	for key, path := range settingsFields {
		if err := nestState(values, path, values[key]); err != nil {
			return err
		}
		delete(values, key)
	}
	// Write migrated data before removing its legacy copy from settings.json.
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := writeSettingsJSON(filepath.Join(filepath.Dir(s.path), "state.json"), data); err != nil {
		return err
	}
	data, err = json.MarshalIndent(favourites, "", "  ")
	if err != nil {
		return err
	}
	if err := writeSettingsJSON(filepath.Join(filepath.Dir(s.path), "favourites.json"), data); err != nil {
		return err
	}
	data, err = json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	return writeSettingsJSON(s.path, data)
}

func writeSettingsJSON(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
