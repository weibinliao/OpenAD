//go:build windows

package scanner

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func (scanner *NTFSScanner) readPermissions(path string) ([]Permission, error) {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return nil, fmt.Errorf("get security info for %s: %w", path, err)
	}

	dacl, _, err := sd.DACL()
	if err != nil {
		return nil, fmt.Errorf("get DACL for %s: %w", path, err)
	}

	if dacl == nil || dacl.AceCount == 0 {
		return nil, nil
	}

	permissions := make([]Permission, 0, dacl.AceCount)
	for i := uint16(0); i < dacl.AceCount; i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(i), &ace); err != nil {
			return nil, fmt.Errorf("read ACE %d for %s: %w", i, path, err)
		}

		if ace == nil {
			continue
		}

		sid := (*windows.SID)(unsafe.Pointer(uintptr(unsafe.Pointer(ace)) + unsafe.Offsetof(ace.SidStart)))
		trustee, trusteeSID, accountType := scanner.resolveSID(sid)
		rights := scanner.parseRights(ace.Mask)
		riskLevel := scanner.classifyRiskLevel(rights, ace.Mask)

		permissions = append(permissions, Permission{
			Path:        path,
			Trustee:     trustee,
			TrusteeSID:  trusteeSID,
			Rights:      rights,
			Type:        scanner.parseAceType(ace.Header.AceType),
			Inherited:   ace.Header.AceFlags&windows.INHERITED_ACE != 0,
			Source:      scanner.getInheritanceSource(ace.Header.AceFlags),
			AppliesTo:   scanner.parseAppliesTo(ace.Header.AceFlags),
			AccountType: accountType,
			AccessMask:  fmt.Sprintf("0x%08X", uint32(ace.Mask)),
			RiskLevel:   riskLevel,
			ParentDelta: scanner.describeParentDelta(ace.Header.AceFlags),
		})
	}

	return permissions, nil
}

func (scanner *NTFSScanner) resolveSID(sid *windows.SID) (string, string, string) {
	if sid == nil {
		return "Unknown", "", "Unknown"
	}

	sidValue := sid.String()
	account, domain, accountType, err := sid.LookupAccount("")
	if err != nil {
		return sidValue, sidValue, "Unknown"
	}

	if domain == "" {
		return account, sidValue, scanner.parseAccountType(accountType)
	}

	return fmt.Sprintf("%s\\%s", domain, account), sidValue, scanner.parseAccountType(accountType)
}

func (scanner *NTFSScanner) parseRights(mask windows.ACCESS_MASK) string {
	rights := make([]string, 0, 4)

	if mask&windows.GENERIC_ALL != 0 {
		rights = append(rights, "Full Control")
	} else {
		hasRead := mask&windows.GENERIC_READ != 0 || mask&windows.FILE_GENERIC_READ == windows.FILE_GENERIC_READ
		hasWrite := mask&windows.GENERIC_WRITE != 0 || mask&windows.FILE_GENERIC_WRITE == windows.FILE_GENERIC_WRITE
		hasExecute := mask&windows.GENERIC_EXECUTE != 0 || mask&windows.FILE_GENERIC_EXECUTE == windows.FILE_GENERIC_EXECUTE

		if hasRead && hasExecute {
			rights = append(rights, "Read and Execute")
			hasRead = false
			hasExecute = false
		}
		if hasRead {
			rights = append(rights, "Read")
		}
		if hasWrite {
			rights = append(rights, "Write")
		}
		if hasExecute {
			rights = append(rights, "Execute")
		}

		extraChecks := []struct {
			mask  windows.ACCESS_MASK
			label string
		}{
			{mask: windows.DELETE, label: "Delete"},
			{mask: windows.WRITE_DAC, label: "Change Permissions"},
			{mask: windows.WRITE_OWNER, label: "Take Ownership"},
			{mask: windows.SYNCHRONIZE, label: "Synchronize"},
		}
		for _, item := range extraChecks {
			if mask&item.mask != 0 && !containsString(rights, item.label) {
				rights = append(rights, item.label)
			}
		}
	}

	if len(rights) == 0 {
		return fmt.Sprintf("0x%x", uint32(mask))
	}

	return strings.Join(rights, ", ")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}

	return false
}

func (scanner *NTFSScanner) parseAceType(aceType uint8) string {
	switch aceType {
	case windows.ACCESS_ALLOWED_ACE_TYPE:
		return "Allow"
	case windows.ACCESS_DENIED_ACE_TYPE:
		return "Deny"
	default:
		return "Unknown"
	}
}

func (scanner *NTFSScanner) getInheritanceSource(flags uint8) string {
	if flags&windows.INHERITED_ACE != 0 {
		return "Inherited"
	}

	return "Explicit"
}

func (scanner *NTFSScanner) parseAppliesTo(flags uint8) string {
	containerInherit := flags&windows.CONTAINER_INHERIT_ACE != 0
	objectInherit := flags&windows.OBJECT_INHERIT_ACE != 0
	inheritOnly := flags&windows.INHERIT_ONLY_ACE != 0
	noPropagate := flags&windows.NO_PROPAGATE_INHERIT_ACE != 0

	var appliesTo string
	switch {
	case inheritOnly && containerInherit && objectInherit:
		appliesTo = "Subfolders and Files Only"
	case inheritOnly && containerInherit:
		appliesTo = "Subfolders Only"
	case inheritOnly && objectInherit:
		appliesTo = "Files Only"
	case containerInherit && objectInherit:
		appliesTo = "This Folder, Subfolders and Files"
	case containerInherit:
		appliesTo = "This Folder and Subfolders"
	case objectInherit:
		appliesTo = "This Folder and Files"
	default:
		appliesTo = "This Folder Only"
	}

	if noPropagate {
		return appliesTo + " (No Propagate)"
	}

	return appliesTo
}

func (scanner *NTFSScanner) parseAccountType(accountType uint32) string {
	switch accountType {
	case windows.SidTypeUser:
		return "User"
	case windows.SidTypeGroup:
		return "Group"
	case windows.SidTypeDomain:
		return "Domain"
	case windows.SidTypeAlias:
		return "Alias"
	case windows.SidTypeWellKnownGroup:
		return "WellKnownGroup"
	case windows.SidTypeDeletedAccount:
		return "DeletedAccount"
	case windows.SidTypeInvalid:
		return "Invalid"
	case windows.SidTypeUnknown:
		return "Unknown"
	case windows.SidTypeComputer:
		return "Computer"
	case windows.SidTypeLabel:
		return "Label"
	default:
		return "Unknown"
	}
}

func (scanner *NTFSScanner) classifyRiskLevel(rights string, mask windows.ACCESS_MASK) string {
	value := strings.ToLower(strings.TrimSpace(rights))
	switch {
	case value == "":
		return "unknown"
	case strings.Contains(value, "full control"),
		strings.Contains(value, "take ownership"),
		strings.Contains(value, "change permissions"),
		strings.Contains(value, "delete"),
		strings.Contains(value, "write"),
		mask&windows.GENERIC_ALL != 0,
		mask&windows.WRITE_DAC != 0,
		mask&windows.WRITE_OWNER != 0:
		return "high"
	case strings.Contains(value, "execute"),
		mask&windows.GENERIC_EXECUTE != 0:
		return "medium"
	default:
		return "low"
	}
}

func (scanner *NTFSScanner) describeParentDelta(flags uint8) string {
	if flags&windows.INHERITED_ACE != 0 {
		return "Inherited from Parent"
	}
	if flags&(windows.CONTAINER_INHERIT_ACE|windows.OBJECT_INHERIT_ACE|windows.INHERIT_ONLY_ACE|windows.NO_PROPAGATE_INHERIT_ACE) != 0 {
		return "Explicit Inheritance Override"
	}
	return "Explicit on Current Item"
}
