package sdui

type ComponentType string

const (
	ComponentRow      ComponentType = "row"
	ComponentColumn   ComponentType = "column"
	ComponentCard     ComponentType = "card"
	ComponentListTile ComponentType = "list_tile"
	ComponentText     ComponentType = "text"
	ComponentImage    ComponentType = "image"
	ComponentButton   ComponentType = "button"
	ComponentSpacer   ComponentType = "spacer"
	ComponentDivider  ComponentType = "divider"
)

type ComponentSpec struct {
	Type        ComponentType          `json:"type"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]PropertySpec `json:"properties,omitempty"`
}

type PropertySpec struct {
	Type        string `json:"type"` // "string", "int", "bool", "color", "enum", "list"
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

var Registry = map[ComponentType]ComponentSpec{
	ComponentRow: {
		Type: ComponentRow,
		Description: "Horizontal layout container",
		Properties: map[string]PropertySpec{
			"children": {Type: "list", Required: true},
			"mainAxisAlignment": {Type: "enum", Description: "start, center, end, spaceBetween, spaceAround, spaceEvenly"},
			"crossAxisAlignment": {Type: "enum", Description: "start, center, end, stretch"},
		},
	},
	ComponentColumn: {
		Type: ComponentColumn,
		Description: "Vertical layout container",
		Properties: map[string]PropertySpec{
			"children": {Type: "list", Required: true},
			"mainAxisAlignment": {Type: "enum", Description: "start, center, end, spaceBetween, spaceAround, spaceEvenly"},
			"crossAxisAlignment": {Type: "enum", Description: "start, center, end, stretch"},
		},
	},
	ComponentCard: {
		Type: ComponentCard,
		Description: "Material-style card container",
		Properties: map[string]PropertySpec{
			"child": {Type: "component", Required: true},
			"elevation": {Type: "float"},
			"padding": {Type: "float"},
			"margin": {Type: "float"},
			"borderRadius": {Type: "float"},
			"color": {Type: "color"},
		},
	},
	ComponentListTile: {
		Type: ComponentListTile,
		Description: "Standardized list item",
		Properties: map[string]PropertySpec{
			"title": {Type: "string", Required: true},
			"subtitle": {Type: "string"},
			"leading": {Type: "component"},
			"trailing": {Type: "component"},
			"onTap": {Type: "action"},
		},
	},
	ComponentText: {
		Type: ComponentText,
		Description: "Basic text element",
		Properties: map[string]PropertySpec{
			"value": {Type: "string", Required: true},
			"style": {Type: "enum", Description: "h1, h2, h3, body, caption, button"},
			"color": {Type: "color"},
			"align": {Type: "enum", Description: "left, center, right, justify"},
		},
	},
	ComponentImage: {
		Type: ComponentImage,
		Description: "Image display element",
		Properties: map[string]PropertySpec{
			"url": {Type: "string", Required: true},
			"width": {Type: "float"},
			"height": {Type: "float"},
			"fit": {Type: "enum", Description: "cover, contain, fill, fitWidth, fitHeight"},
			"borderRadius": {Type: "float"},
		},
	},
	ComponentButton: {
		Type: ComponentButton,
		Description: "Interactive button",
		Properties: map[string]PropertySpec{
			"label": {Type: "string", Required: true},
			"type": {Type: "enum", Description: "primary, secondary, outline, text"},
			"onTap": {Type: "action", Required: true},
			"icon": {Type: "string"},
		},
	},
}
