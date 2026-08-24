// Package tui provides the keyboard-driven Bubble Tea interface. Environment selection itself is
// a charm.land/huh/v2 form (see model.go's newSelectField/newForm); the outer Model is a thin
// Bubble Tea wrapper around it, responsible only for application-level shortcuts (view/edit/
// reload/install/quit) and the small hand-rolled y/n confirmation dialogs those can raise.
package tui
