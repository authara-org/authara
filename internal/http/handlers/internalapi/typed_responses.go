package internalapi

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
)

func apiErrorBody(code response.ErrorCode, message string) contract.ErrorResponse {
	return contract.ErrorResponse{
		Error: contract.APIError{
			Code:    string(code),
			Message: message,
		},
	}
}

func getPublicCapabilitiesError(code response.ErrorCode, message string) contract.GetPublicCapabilitiesResponseObject {
	spec := mustRouteError(GetPublicCapabilitiesErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusUnauthorized:
		return contract.GetPublicCapabilities401JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	default:
		return contract.GetPublicCapabilities500JSONResponse(body)
	}
}

func createInternalOrganizationError(code response.ErrorCode, message string) contract.CreateInternalOrganizationResponseObject {
	spec := mustRouteError(CreateInternalOrganizationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.CreateInternalOrganization400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.CreateInternalOrganization401JSONResponse(body)
	case http.StatusForbidden:
		return contract.CreateInternalOrganization403JSONResponse(body)
	case http.StatusNotFound:
		return contract.CreateInternalOrganization404JSONResponse(body)
	default:
		return contract.CreateInternalOrganization500JSONResponse(body)
	}
}

func deleteInternalOrganizationError(code response.ErrorCode, message string) contract.DeleteInternalOrganizationResponseObject {
	spec := mustRouteError(DeleteInternalOrganizationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.DeleteInternalOrganization400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.DeleteInternalOrganization401JSONResponse(body)
	case http.StatusForbidden:
		return contract.DeleteInternalOrganization403JSONResponse(body)
	case http.StatusNotFound:
		return contract.DeleteInternalOrganization404JSONResponse(body)
	case http.StatusConflict:
		return contract.DeleteInternalOrganization409JSONResponse(body)
	default:
		return contract.DeleteInternalOrganization500JSONResponse(body)
	}
}

func removeInternalOrganizationMemberError(code response.ErrorCode, message string) contract.RemoveInternalOrganizationMemberResponseObject {
	spec := mustRouteError(RemoveInternalOrganizationMemberErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.RemoveInternalOrganizationMember400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.RemoveInternalOrganizationMember401JSONResponse(body)
	case http.StatusForbidden:
		return contract.RemoveInternalOrganizationMember403JSONResponse(body)
	case http.StatusNotFound:
		return contract.RemoveInternalOrganizationMember404JSONResponse(body)
	case http.StatusConflict:
		return contract.RemoveInternalOrganizationMember409JSONResponse(body)
	default:
		return contract.RemoveInternalOrganizationMember500JSONResponse(body)
	}
}

func transferInternalOrganizationOwnershipError(code response.ErrorCode, message string) contract.TransferInternalOrganizationOwnershipResponseObject {
	spec := mustRouteError(TransferInternalOrganizationOwnershipErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.TransferInternalOrganizationOwnership400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.TransferInternalOrganizationOwnership401JSONResponse(body)
	case http.StatusForbidden:
		return contract.TransferInternalOrganizationOwnership403JSONResponse(body)
	case http.StatusNotFound:
		return contract.TransferInternalOrganizationOwnership404JSONResponse(body)
	case http.StatusConflict:
		return contract.TransferInternalOrganizationOwnership409JSONResponse(body)
	default:
		return contract.TransferInternalOrganizationOwnership500JSONResponse(body)
	}
}

func deleteInternalUserError(code response.ErrorCode, message string) contract.DeleteInternalUserResponseObject {
	spec := mustRouteError(DeleteInternalUserErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.DeleteInternalUser400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.DeleteInternalUser401JSONResponse(body)
	case http.StatusNotFound:
		return contract.DeleteInternalUser404JSONResponse(body)
	case http.StatusConflict:
		return contract.DeleteInternalUser409JSONResponse(body)
	default:
		return contract.DeleteInternalUser500JSONResponse(body)
	}
}

func getPublicOrganizationError(code response.ErrorCode, message string) contract.GetPublicOrganizationResponseObject {
	spec := mustRouteError(GetPublicOrganizationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.GetPublicOrganization400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.GetPublicOrganization401JSONResponse(body)
	case http.StatusForbidden:
		return contract.GetPublicOrganization403JSONResponse(body)
	case http.StatusNotFound:
		return contract.GetPublicOrganization404JSONResponse(body)
	default:
		return contract.GetPublicOrganization500JSONResponse(body)
	}
}

func updatePublicOrganizationError(code response.ErrorCode, message string) contract.UpdatePublicOrganizationResponseObject {
	spec := mustRouteError(UpdatePublicOrganizationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.UpdatePublicOrganization400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.UpdatePublicOrganization401JSONResponse(body)
	case http.StatusForbidden:
		return contract.UpdatePublicOrganization403JSONResponse(body)
	case http.StatusNotFound:
		return contract.UpdatePublicOrganization404JSONResponse(body)
	default:
		return contract.UpdatePublicOrganization500JSONResponse(body)
	}
}

func listPublicOrganizationMembersError(code response.ErrorCode, message string) contract.ListPublicOrganizationMembersResponseObject {
	spec := mustRouteError(ListPublicOrganizationMembersErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.ListPublicOrganizationMembers400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.ListPublicOrganizationMembers401JSONResponse(body)
	case http.StatusForbidden:
		return contract.ListPublicOrganizationMembers403JSONResponse(body)
	case http.StatusNotFound:
		return contract.ListPublicOrganizationMembers404JSONResponse(body)
	default:
		return contract.ListPublicOrganizationMembers500JSONResponse(body)
	}
}

func getPublicOrganizationMemberError(code response.ErrorCode, message string) contract.GetPublicOrganizationMemberResponseObject {
	spec := mustRouteError(GetPublicOrganizationMemberErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.GetPublicOrganizationMember400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.GetPublicOrganizationMember401JSONResponse(body)
	case http.StatusForbidden:
		return contract.GetPublicOrganizationMember403JSONResponse(body)
	case http.StatusNotFound:
		return contract.GetPublicOrganizationMember404JSONResponse(body)
	default:
		return contract.GetPublicOrganizationMember500JSONResponse(body)
	}
}

func listPublicUserMembershipsError(code response.ErrorCode, message string) contract.ListPublicUserMembershipsResponseObject {
	spec := mustRouteError(ListPublicUserMembershipsErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.ListPublicUserMemberships400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.ListPublicUserMemberships401JSONResponse(body)
	case http.StatusForbidden:
		return contract.ListPublicUserMemberships403JSONResponse(body)
	case http.StatusNotFound:
		return contract.ListPublicUserMemberships404JSONResponse(body)
	default:
		return contract.ListPublicUserMemberships500JSONResponse(body)
	}
}

func listPublicOrganizationInvitationsError(code response.ErrorCode, message string) contract.ListPublicOrganizationInvitationsResponseObject {
	spec := mustRouteError(ListPublicOrganizationInvitationsErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.ListPublicOrganizationInvitations400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.ListPublicOrganizationInvitations401JSONResponse(body)
	case http.StatusForbidden:
		return contract.ListPublicOrganizationInvitations403JSONResponse(body)
	case http.StatusNotFound:
		return contract.ListPublicOrganizationInvitations404JSONResponse(body)
	default:
		return contract.ListPublicOrganizationInvitations500JSONResponse(body)
	}
}

func getPublicOrganizationInvitationError(code response.ErrorCode, message string) contract.GetPublicOrganizationInvitationResponseObject {
	spec := mustRouteError(GetPublicOrganizationInvitationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.GetPublicOrganizationInvitation400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.GetPublicOrganizationInvitation401JSONResponse(body)
	case http.StatusForbidden:
		return contract.GetPublicOrganizationInvitation403JSONResponse(body)
	case http.StatusNotFound:
		return contract.GetPublicOrganizationInvitation404JSONResponse(body)
	default:
		return contract.GetPublicOrganizationInvitation500JSONResponse(body)
	}
}

func revokePublicOrganizationInvitationError(code response.ErrorCode, message string) contract.RevokePublicOrganizationInvitationResponseObject {
	spec := mustRouteError(RevokePublicOrganizationInvitationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.RevokePublicOrganizationInvitation400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.RevokePublicOrganizationInvitation401JSONResponse(body)
	case http.StatusForbidden:
		return contract.RevokePublicOrganizationInvitation403JSONResponse(body)
	case http.StatusNotFound:
		return contract.RevokePublicOrganizationInvitation404JSONResponse(body)
	case http.StatusConflict:
		return contract.RevokePublicOrganizationInvitation409JSONResponse(body)
	default:
		return contract.RevokePublicOrganizationInvitation500JSONResponse(body)
	}
}

func createInternalOrganizationInvitationError(code response.ErrorCode, message string) contract.CreateInternalOrganizationInvitationResponseObject {
	spec := mustRouteError(CreateInternalOrganizationInvitationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.CreateInternalOrganizationInvitation400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.CreateInternalOrganizationInvitation401JSONResponse(body)
	case http.StatusForbidden:
		return contract.CreateInternalOrganizationInvitation403JSONResponse(body)
	case http.StatusNotFound:
		return contract.CreateInternalOrganizationInvitation404JSONResponse(body)
	case http.StatusConflict:
		return contract.CreateInternalOrganizationInvitation409JSONResponse(body)
	default:
		return contract.CreateInternalOrganizationInvitation500JSONResponse(body)
	}
}

func resendInternalOrganizationInvitationError(code response.ErrorCode, message string) contract.ResendInternalOrganizationInvitationResponseObject {
	spec := mustRouteError(ResendInternalOrganizationInvitationErrors, code)
	body := apiErrorBody(spec.Code, message)
	switch spec.Status {
	case http.StatusBadRequest:
		return contract.ResendInternalOrganizationInvitation400JSONResponse{ErrorJSONResponse: contract.ErrorJSONResponse(body)}
	case http.StatusUnauthorized:
		return contract.ResendInternalOrganizationInvitation401JSONResponse(body)
	case http.StatusForbidden:
		return contract.ResendInternalOrganizationInvitation403JSONResponse(body)
	case http.StatusNotFound:
		return contract.ResendInternalOrganizationInvitation404JSONResponse(body)
	case http.StatusConflict:
		return contract.ResendInternalOrganizationInvitation409JSONResponse(body)
	default:
		return contract.ResendInternalOrganizationInvitation500JSONResponse(body)
	}
}
