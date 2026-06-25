package internalapi

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/response"
)

type RouteContractSpec struct {
	Method string
	Path   string
	Errors map[response.ErrorCode]response.ErrorSpec
}

const (
	codeUserNotFound              response.ErrorCode = "user_not_found"
	codeOrganizationNotFound      response.ErrorCode = "organization_not_found"
	codeMembershipNotFound        response.ErrorCode = "membership_not_found"
	codeInvitationNotFound        response.ErrorCode = "invitation_not_found"
	codeActorNotMember            response.ErrorCode = "actor_not_member"
	codeActorNotAllowed           response.ErrorCode = "actor_not_allowed"
	codeAlreadyMember             response.ErrorCode = "already_member"
	codeInvitationAlreadyPending  response.ErrorCode = "invitation_already_pending"
	codeInvitationAlreadyAccepted response.ErrorCode = "invitation_already_accepted"
	codeInvitationRevoked         response.ErrorCode = "invitation_revoked"
	codeInvitationExpired         response.ErrorCode = "invitation_expired"
)

var CapabilitiesGetErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized: {Status: http.StatusUnauthorized, Code: response.CodeUnauthorized},
}

var CreateOrganizationErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized:   {Status: http.StatusUnauthorized, Code: response.CodeUnauthorized},
	response.CodeInvalidRequest: {Status: http.StatusBadRequest, Code: response.CodeInvalidRequest},
	response.CodeForbidden:      {Status: http.StatusForbidden, Code: response.CodeForbidden},
	codeUserNotFound:            {Status: http.StatusNotFound, Code: codeUserNotFound},
	response.CodeInternalError:  {Status: http.StatusInternalServerError, Code: response.CodeInternalError},
}

var OrganizationErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized:   {Status: http.StatusUnauthorized, Code: response.CodeUnauthorized},
	response.CodeInvalidRequest: {Status: http.StatusBadRequest, Code: response.CodeInvalidRequest},
	codeOrganizationNotFound:    {Status: http.StatusNotFound, Code: codeOrganizationNotFound},
	response.CodeInternalError:  {Status: http.StatusInternalServerError, Code: response.CodeInternalError},
}

var OrganizationMembersGetErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized:   {Status: http.StatusUnauthorized, Code: response.CodeUnauthorized},
	response.CodeInvalidRequest: {Status: http.StatusBadRequest, Code: response.CodeInvalidRequest},
	codeOrganizationNotFound:    {Status: http.StatusNotFound, Code: codeOrganizationNotFound},
	response.CodeInternalError:  {Status: http.StatusInternalServerError, Code: response.CodeInternalError},
}

var OrganizationMemberErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized:   {Status: http.StatusUnauthorized, Code: response.CodeUnauthorized},
	response.CodeInvalidRequest: {Status: http.StatusBadRequest, Code: response.CodeInvalidRequest},
	codeOrganizationNotFound:    {Status: http.StatusNotFound, Code: codeOrganizationNotFound},
	codeMembershipNotFound:      {Status: http.StatusNotFound, Code: codeMembershipNotFound},
	response.CodeInternalError:  {Status: http.StatusInternalServerError, Code: response.CodeInternalError},
}

var OrganizationInvitationsGetErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized:   {Status: http.StatusUnauthorized, Code: response.CodeUnauthorized},
	response.CodeInvalidRequest: {Status: http.StatusBadRequest, Code: response.CodeInvalidRequest},
	response.CodeForbidden:      {Status: http.StatusForbidden, Code: response.CodeForbidden},
	codeOrganizationNotFound:    {Status: http.StatusNotFound, Code: codeOrganizationNotFound},
	response.CodeInternalError:  {Status: http.StatusInternalServerError, Code: response.CodeInternalError},
}

var CreateOrganizationInvitationErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized:    {Status: http.StatusUnauthorized, Code: response.CodeUnauthorized},
	codeActorNotMember:           {Status: http.StatusForbidden, Code: codeActorNotMember},
	codeActorNotAllowed:          {Status: http.StatusForbidden, Code: codeActorNotAllowed},
	codeOrganizationNotFound:     {Status: http.StatusNotFound, Code: codeOrganizationNotFound},
	codeAlreadyMember:            {Status: http.StatusConflict, Code: codeAlreadyMember},
	codeInvitationAlreadyPending: {Status: http.StatusConflict, Code: codeInvitationAlreadyPending},
	response.CodeInvalidRequest:  {Status: http.StatusBadRequest, Code: response.CodeInvalidRequest},
	response.CodeInternalError:   {Status: http.StatusInternalServerError, Code: response.CodeInternalError},
}

var OrganizationInvitationGetErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized:   {Status: http.StatusUnauthorized, Code: response.CodeUnauthorized},
	response.CodeInvalidRequest: {Status: http.StatusBadRequest, Code: response.CodeInvalidRequest},
	response.CodeForbidden:      {Status: http.StatusForbidden, Code: response.CodeForbidden},
	codeInvitationNotFound:      {Status: http.StatusNotFound, Code: codeInvitationNotFound},
	response.CodeInternalError:  {Status: http.StatusInternalServerError, Code: response.CodeInternalError},
}

var RevokeOrganizationInvitationErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized:     {Status: http.StatusUnauthorized, Code: response.CodeUnauthorized},
	response.CodeInvalidRequest:   {Status: http.StatusBadRequest, Code: response.CodeInvalidRequest},
	response.CodeForbidden:        {Status: http.StatusForbidden, Code: response.CodeForbidden},
	codeOrganizationNotFound:      {Status: http.StatusNotFound, Code: codeOrganizationNotFound},
	codeInvitationNotFound:        {Status: http.StatusNotFound, Code: codeInvitationNotFound},
	codeUserNotFound:              {Status: http.StatusNotFound, Code: codeUserNotFound},
	codeInvitationAlreadyAccepted: {Status: http.StatusConflict, Code: codeInvitationAlreadyAccepted},
	codeInvitationRevoked:         {Status: http.StatusConflict, Code: codeInvitationRevoked},
	codeInvitationExpired:         {Status: http.StatusConflict, Code: codeInvitationExpired},
	response.CodeInternalError:    {Status: http.StatusInternalServerError, Code: response.CodeInternalError},
}

var UserMembershipsGetErrors = map[response.ErrorCode]response.ErrorSpec{
	response.CodeUnauthorized:   {Status: http.StatusUnauthorized, Code: response.CodeUnauthorized},
	response.CodeInvalidRequest: {Status: http.StatusBadRequest, Code: response.CodeInvalidRequest},
	codeUserNotFound:            {Status: http.StatusNotFound, Code: codeUserNotFound},
	response.CodeInternalError:  {Status: http.StatusInternalServerError, Code: response.CodeInternalError},
}

var InternalAPIRouteSpecs = []RouteContractSpec{
	{Method: http.MethodGet, Path: "/auth/internal/v1/capabilities", Errors: CapabilitiesGetErrors},
	{Method: http.MethodPost, Path: "/auth/internal/v1/organizations", Errors: CreateOrganizationErrors},
	{Method: http.MethodGet, Path: "/auth/internal/v1/organizations/{organizationID}", Errors: OrganizationErrors},
	{Method: http.MethodPatch, Path: "/auth/internal/v1/organizations/{organizationID}", Errors: OrganizationErrors},
	{Method: http.MethodGet, Path: "/auth/internal/v1/organizations/{organizationID}/members", Errors: OrganizationMembersGetErrors},
	{Method: http.MethodGet, Path: "/auth/internal/v1/organizations/{organizationID}/members/{userID}", Errors: OrganizationMemberErrors},
	{Method: http.MethodGet, Path: "/auth/internal/v1/organizations/{organizationID}/invitations", Errors: OrganizationInvitationsGetErrors},
	{Method: http.MethodPost, Path: "/auth/internal/v1/organizations/{organizationID}/invitations", Errors: CreateOrganizationInvitationErrors},
	{Method: http.MethodGet, Path: "/auth/internal/v1/organizations/{organizationID}/invitations/{invitationID}", Errors: OrganizationInvitationGetErrors},
	{Method: http.MethodPost, Path: "/auth/internal/v1/organizations/{organizationID}/invitations/{invitationID}/revoke", Errors: RevokeOrganizationInvitationErrors},
	{Method: http.MethodGet, Path: "/auth/internal/v1/users/{userID}/memberships", Errors: UserMembershipsGetErrors},
}
