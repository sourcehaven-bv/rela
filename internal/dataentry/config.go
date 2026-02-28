// Package dataentry provides a config-driven data entry web application
// built on top of rela's metamodel system. It reads a data-entry.yaml config
// file alongside a rela project and serves an interactive UI for CRUD operations
// on entities stored as markdown files.
//
// Configuration types and validation logic live in the dataentryconfig package
// so that the CLI can validate configs without importing the full web layer.
// This file re-exports those types for backward compatibility.
package dataentry

import "github.com/Sourcehaven-BV/rela/internal/dataentryconfig"

// Widget and direction constants — re-exported from dataentryconfig.
const (
	WidgetText        = dataentryconfig.WidgetText
	WidgetSelect      = dataentryconfig.WidgetSelect
	WidgetMultiSelect = dataentryconfig.WidgetMultiSelect
	WidgetCheckbox    = dataentryconfig.WidgetCheckbox
	WidgetTextarea    = dataentryconfig.WidgetTextarea
	WidgetNumber      = dataentryconfig.WidgetNumber
	WidgetDate        = dataentryconfig.WidgetDate

	DirectionIncoming = dataentryconfig.DirectionIncoming
	DirectionOutgoing = dataentryconfig.DirectionOutgoing
)

// Config type aliases — re-exported from dataentryconfig for backward compatibility.
type (
	Config           = dataentryconfig.Config
	AppConfig        = dataentryconfig.AppConfig
	Form             = dataentryconfig.Form
	SidePanelConfig  = dataentryconfig.SidePanelConfig
	FormField        = dataentryconfig.FormField
	FormRelation     = dataentryconfig.FormRelation
	RelationProperty = dataentryconfig.RelationProperty
	List             = dataentryconfig.List
	ListColumn       = dataentryconfig.ListColumn
	SortSpec         = dataentryconfig.SortSpec
	FilterConfig     = dataentryconfig.FilterConfig
	FilterControl    = dataentryconfig.FilterControl
	Kanban           = dataentryconfig.Kanban
	KanbanColumn     = dataentryconfig.KanbanColumn
	KanbanSwimlane   = dataentryconfig.KanbanSwimlane
	KanbanCard       = dataentryconfig.KanbanCard
	NavigationEntry  = dataentryconfig.NavigationEntry
	UIState          = dataentryconfig.UIState
	UserDefaults     = dataentryconfig.UserDefaults
	DefaultOverride  = dataentryconfig.DefaultOverride
	DashboardConfig  = dataentryconfig.DashboardConfig
	DashboardCard    = dataentryconfig.DashboardCard
	ViewConfig       = dataentryconfig.ViewConfig
	ViewEntry        = dataentryconfig.ViewEntry
	ViewTraverse     = dataentryconfig.ViewTraverse
	ViewSection      = dataentryconfig.ViewSection
	ViewSectionField = dataentryconfig.ViewSectionField
	CommandConfig  = dataentryconfig.CommandConfig
	CommandScope   = dataentryconfig.CommandScope
	DocumentConfig = dataentryconfig.DocumentConfig
)
