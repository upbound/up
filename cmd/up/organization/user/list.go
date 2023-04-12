// Copyright 2023 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package user

import (
	"context"
	"sort"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

type Member struct {
	Member organizations.Member
	Invite organizations.Invite
}

const (
	statusActive  = "ACTIVE"
	statusInvited = "INVITED"
)

var listFieldNames = []string{"USERNAME", "NAME", "EMAIL", "PERMISSION", "STATUS"}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd lists users of an organization.
// It lists both members and invites.
type listCmd struct {
	OrgName string `arg:"" required:"" help:"Name of the organization."`
}

// Run executes the list command.
func (c *listCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	orgID, err := oc.GetOrgID(context.Background(), c.OrgName)
	if err != nil {
		return err
	}
	members, err := oc.ListMembers(context.Background(), orgID)
	if err != nil {
		return err
	}
	invites, err := oc.ListInvites(context.Background(), orgID)
	if err != nil {
		return err
	}

	// Create a full list of members & invites, sorted by username or email.
	allMembers := make([]Member, len(invites)+len(members))
	for i, invite := range invites {
		allMembers[i] = Member{Invite: invite}
	}
	for i, member := range members {
		allMembers[i+len(invites)] = Member{Member: member}
	}

	sort.SliceStable(allMembers, func(i, j int) bool {
		if allMembers[i].Member.User.Username != "" && allMembers[j].Member.User.Username != "" {
			return allMembers[i].Member.User.Username < allMembers[j].Member.User.Username
		}
		if allMembers[i].Member.User.Username != "" {
			return true
		}
		if allMembers[j].Member.User.Username != "" {
			return false
		}
		return allMembers[i].Invite.Email < allMembers[j].Invite.Email
	})

	return printer.Print(allMembers, listFieldNames, extractMemberFields)
}

func extractMemberFields(obj any) []string {
	m := obj.(Member)
	// If the user name exists, this is a member, not an invite.
	if m.Member.User.Username != "" {
		return []string{m.Member.User.Username, m.Member.User.Name, m.Member.User.Email, string(m.Member.Permission), statusActive}
	}
	// invites don't have usernames or names, so those are left blank.
	return []string{"", "", m.Invite.Email, string(m.Invite.Permission), statusInvited}
}
