package main

import (
	"flag"
	"fmt"
	"reflect"
	"strings"

	"github.com/muesli/reflow/indent"
	"github.com/muesli/reflow/wordwrap"
)

// mode describes a group of related CLI flags and their help presentation.
type mode struct {
	name        string
	description string
	usage       string       // e.g. "run [flags] <task>" — shown before flags
	examples    []string     // e.g. "run -session=<name> -status" — shown before flags
	inv         any          // struct with flag/pos tags; the invocation type
	after       func() string // if set, rendered after flags
}

// flagInfo holds the parsed metadata for a single struct field with a flag tag.
type flagInfo struct {
	name     string
	usage    string
	defValue string
	kind     string // "string", "bool", "multi"
}

// posInfo holds the parsed metadata for a positional argument field.
type posInfo struct {
	name     string
	usage    string
	required bool
}

// registerFlags walks all modes, reflects over their inv structs, and
// registers each flag with the stdlib flag package. When the same flag
// name appears in multiple modes, it is registered once and all struct
// fields sharing that name are synced after parsing via syncSharedFlags.
func registerFlags(modes []mode) {
	registered := map[string]bool{}
	for i := range modes {
		m := &modes[i]
		if m.inv == nil {
			continue
		}
		rv := reflect.ValueOf(m.inv)
		if rv.Kind() != reflect.Pointer {
			panic(fmt.Sprintf("mode %q inv must be a pointer to a struct", m.name))
		}
		rv = rv.Elem()
		rt := rv.Type()
		for j := 0; j < rt.NumField(); j++ {
			field := rt.Field(j)
			flagName := field.Tag.Get("flag")
			if flagName == "" || registered[flagName] {
				continue
			}
			registered[flagName] = true
			usage := field.Tag.Get("usage")
			defValue := field.Tag.Get("default")
			fv := rv.Field(j)
			switch field.Type.Kind() {
			case reflect.String:
				flag.StringVar(fv.Addr().Interface().(*string), flagName, defValue, usage)
			case reflect.Bool:
				flag.BoolVar(fv.Addr().Interface().(*bool), flagName, false, usage)
			case reflect.Slice:
				if field.Type.Elem().Kind() == reflect.String {
					ptr := fv.Addr().Interface().(*[]string)
					flag.Func(flagName, usage, func(s string) error {
						*ptr = append(*ptr, s)
						return nil
					})
				}
			}
		}
	}
}

// syncSharedFlags copies flag values to all inv struct fields that share
// the same flag name. Call after flag.Parse().
func syncSharedFlags(modes []mode) {
	// Collect the parsed value for each flag name.
	values := map[string]string{}
	flag.Visit(func(f *flag.Flag) {
		values[f.Name] = f.Value.String()
	})

	// Also collect defaults for unset flags.
	flag.VisitAll(func(f *flag.Flag) {
		if _, set := values[f.Name]; !set {
			values[f.Name] = f.DefValue
		}
	})

	for i := range modes {
		m := &modes[i]
		if m.inv == nil {
			continue
		}
		rv := reflect.ValueOf(m.inv)
		if rv.Kind() == reflect.Pointer {
			rv = rv.Elem()
		}
		rt := rv.Type()
		for j := 0; j < rt.NumField(); j++ {
			field := rt.Field(j)
			flagName := field.Tag.Get("flag")
			if flagName == "" {
				continue
			}
			val, ok := values[flagName]
			if !ok {
				continue
			}
			fv := rv.Field(j)
			switch field.Type.Kind() {
			case reflect.String:
				fv.SetString(val)
			case reflect.Bool:
				fv.SetBool(val == "true")
			}
		}
	}
}

// resolveMode determines which mode the user intended based on the flags
// that were set and whether a positional argument is present. Returns nil
// if no mode matches (caller should show help).
func resolveMode(modes []mode) *mode {
	posArg := flag.Arg(0)

	// Collect which flags were explicitly set.
	setFlags := map[string]bool{}
	flag.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	// For each mode, check if the user's input is a valid invocation.
	var matches []*mode
	for i := range modes {
		m := &modes[i]
		if matchesMode(m, setFlags, posArg) {
			matches = append(matches, m)
		}
	}

	if len(matches) == 1 {
		return matches[0]
	}
	return nil
}

// matchesMode returns true if the given set of flags and positional arg
// form a valid (and complete) invocation for this mode.
func matchesMode(m *mode, setFlags map[string]bool, posArg string) bool {
	if m.inv == nil {
		return false
	}
	rv := reflect.ValueOf(m.inv)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	rt := rv.Type()

	modeFlags := map[string]bool{}
	hasRequiredPos := false
	hasPos := false
	hasRequiredFlag := false
	requiredFlagSet := false

	for j := 0; j < rt.NumField(); j++ {
		field := rt.Field(j)
		if flagName := field.Tag.Get("flag"); flagName != "" {
			modeFlags[flagName] = true
			if field.Tag.Get("required") == "true" {
				hasRequiredFlag = true
				if setFlags[flagName] {
					requiredFlagSet = true
				}
			}
		}
		if field.Tag.Get("pos") != "" {
			hasPos = true
			if field.Tag.Get("required") == "true" {
				hasRequiredPos = true
			}
		}
	}

	// Check that no set flags are outside this mode.
	for name := range setFlags {
		if !modeFlags[name] {
			return false
		}
	}

	// If the mode has a required positional and it's missing, no match.
	if hasRequiredPos && posArg == "" {
		return false
	}

	// If there's a positional arg but this mode doesn't accept one, no match.
	if posArg != "" && !hasPos {
		return false
	}

	// The mode must have some signal — either a required flag is set,
	// a required positional is present, or at least one flag is set.
	if hasRequiredFlag {
		return requiredFlagSet
	}
	if hasRequiredPos {
		return posArg != ""
	}
	// For modes where nothing is required (like Info), at least one
	// flag must be set.
	for name := range setFlags {
		if modeFlags[name] {
			return true
		}
	}
	return false
}

// renderHelp generates the full help text from the mode definitions.
func renderHelp(modes []mode) string {
	var b strings.Builder
	b.WriteString("Run executes collections of tasks defined in tasks.toml files.\n")
	b.WriteString("For documentation and the latest version, please visit GitHub:\n")
	b.WriteString("\n")
	b.WriteString("  https://monks.co/run\n")
	for _, m := range modes {
		b.WriteString("\n")
		b.WriteString(renderModeHelp(m))
	}
	return b.String()
}

// renderModeHelp generates the help section for a single mode.
func renderModeHelp(m mode) string {
	var b strings.Builder
	fmt.Fprintln(&b, headerStyle.Render(m.name))

	if m.description != "" {
		b.WriteString(indent.String(wordwrap.String(m.description, 68), 2) + "\n")
		b.WriteString("\n")
	}

	if m.usage != "" {
		b.WriteString("  " + m.usage + "\n")
		b.WriteString("\n")
	}

	if len(m.examples) > 0 {
		for _, ex := range m.examples {
			b.WriteString("  " + ex + "\n")
		}
		b.WriteString("\n")
	}

	// Render flags from the inv struct.
	if m.inv != nil {
		rv := reflect.ValueOf(m.inv)
		if rv.Kind() == reflect.Pointer {
			rv = rv.Elem()
		}
		rt := rv.Type()
		for j := 0; j < rt.NumField(); j++ {
			field := rt.Field(j)
			flagName := field.Tag.Get("flag")
			if flagName == "" {
				continue
			}
			f := flag.CommandLine.Lookup(flagName)
			if f == nil {
				continue
			}
			renderFlagEntry(&b, f)
		}
	}

	if m.after != nil {
		b.WriteString("\n")
		b.WriteString(m.after())
	}
	return b.String()
}

// renderFlagEntry renders a single flag in the standard indented format.
func renderFlagEntry(b *strings.Builder, f *flag.Flag) {
	fmt.Fprintf(b, "  -%s", f.Name)
	name, usage := flag.UnquoteUsage(f)
	if len(name) > 0 {
		b.WriteString("=")
		b.WriteString(name)
	}
	if !isZeroValue(f, f.DefValue) {
		fmt.Fprintf(b, " (default %q)", f.DefValue)
	}
	b.WriteString("\n")

	usage = strings.ReplaceAll(usage, "\n", "\n    \t")
	usage = wordwrap.String(usage, 52)
	usage = indent.String(usage, 8)
	b.WriteString(usage)
	b.WriteString("\n")
}

// isZeroValue determines whether the string represents the zero
// value for a flag.
func isZeroValue(f *flag.Flag, value string) bool {
	typ := reflect.TypeOf(f.Value)
	var z reflect.Value
	if typ.Kind() == reflect.Pointer {
		z = reflect.New(typ.Elem())
	} else {
		z = reflect.Zero(typ)
	}
	return value == z.Interface().(flag.Value).String()
}

// setPositional sets the positional arg value on the inv struct.
func setPositional(m *mode) {
	posArg := flag.Arg(0)
	if m.inv == nil || posArg == "" {
		return
	}
	rv := reflect.ValueOf(m.inv)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	rt := rv.Type()
	for j := 0; j < rt.NumField(); j++ {
		field := rt.Field(j)
		if field.Tag.Get("pos") != "" {
			rv.Field(j).SetString(posArg)
			return
		}
	}
}
