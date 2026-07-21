package models

type ADObjectType string

const (
	ADObjectTypeUser    ADObjectType = "user"
	ADObjectTypeGroup   ADObjectType = "group"
	ADObjectTypeUnknown ADObjectType = "unknown"
)

type ADPrincipal struct {
	DN             string       `json:"dn"`
	Name           string       `json:"name,omitempty"`
	SAMAccountName string       `json:"sam_account_name,omitempty"`
	SID            string       `json:"sid,omitempty"`
	Email          string       `json:"email,omitempty"`
	FirstName      string       `json:"first_name,omitempty"`
	LastName       string       `json:"last_name,omitempty"`
	Department     string       `json:"department,omitempty"`
	Division       string       `json:"division,omitempty"`
	Domain         string       `json:"domain,omitempty"`
	Type           ADObjectType `json:"type"`
}

type ADGroup struct {
	DN      string        `json:"dn"`
	Name    string        `json:"name,omitempty"`
	Members []ADPrincipal `json:"members,omitempty"`
}

type ADResolvedMember struct {
	ADPrincipal
	Depth int      `json:"depth"`
	Path  []string `json:"path,omitempty"`
}

type ADGroupCycle struct {
	GroupDN string   `json:"group_dn"`
	Path    []string `json:"path"`
}

type ADGroupResolution struct {
	GroupDN          string             `json:"group_dn"`
	Members          []ADResolvedMember `json:"members"`
	NestedGroups     []string           `json:"nested_groups,omitempty"`
	Cycles           []ADGroupCycle     `json:"cycles,omitempty"`
	MaxDepthReached  bool               `json:"max_depth_reached"`
	DirectoryLookups int                `json:"directory_lookups"`
	CacheHits        int                `json:"cache_hits"`
}

type ADTreeNode struct {
	DN          string `json:"dn"`
	Name        string `json:"name"`
	NodeType    string `json:"node_type"` // domain | ou | container | policy | group | user
	HasChildren bool   `json:"has_children"`
}
