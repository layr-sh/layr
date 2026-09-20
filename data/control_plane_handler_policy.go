package data

import (
	"encoding/json"
	"net/http"
	"strings"

	"layr.sh/core"
)

// handleListPolicies lists all Row-Level Security policies on a table.
func (controlPlaneHandler *ControlPlaneHandler) handleListPolicies(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.read") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/policies")
		return
	}
	policies, err := controlPlaneHandler.ddlEngine.ListPolicies(request.Context(), schema, table)
	if err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}
	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ListPoliciesResponse{
		Policies: policies,
		Count:    len(policies),
	})
}

// handleCreatePolicy creates a new Row-Level Security policy on a table.
func (controlPlaneHandler *ControlPlaneHandler) handleCreatePolicy(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/policies")
		return
	}
	var createPolicyInput CreatePolicyInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createPolicyInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if createErr := controlPlaneHandler.ddlEngine.CreatePolicy(request.Context(), schema, table, createPolicyInput); createErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, createErr.Error())
		return
	}

	if controlPlaneHandler.eventBus != nil {
		tableID := schema + "." + table
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
			Schema: schema,
			Table:  table,
			Action: "create_policy",
			Detail: createPolicyInput.Name,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusCreated, CreatePolicyResponse{
		Status: "created",
		Policy: createPolicyInput.Name,
	})
}

// handleDeletePolicy drops a Row-Level Security policy from a table.
func (controlPlaneHandler *ControlPlaneHandler) handleDeletePolicy(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	policyName := controlPlaneHandler.extractPolicyName(request)
	if schema == "" || table == "" || policyName == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/policies/{policy}")
		return
	}
	if dropErr := controlPlaneHandler.ddlEngine.DropPolicy(request.Context(), schema, table, policyName); dropErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, dropErr.Error())
		return
	}

	if controlPlaneHandler.eventBus != nil {
		tableID := schema + "." + table
		controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(tableID, TableUpdatedEventData{
			Schema: schema,
			Table:  table,
			Action: "drop_policy",
			Detail: policyName,
		}))
	}

	controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, DeletePolicyResponse{
		Status: "deleted",
		Policy: policyName,
	})
}

// handleToggleRLS toggles Row-Level Security mode (ENABLE / DISABLE / FORCE / UNFORCE).
func (controlPlaneHandler *ControlPlaneHandler) handleToggleRLS(responseWriter http.ResponseWriter, request *http.Request) {
	if !controlPlaneHandler.checkScope(request, "data:schema.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Insufficient scope permissions for this operation")
		return
	}
	schema, table := controlPlaneHandler.extractSchemaAndTable(request)
	if schema == "" || table == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "URL format must be /api/v1/_/data/tables/{schema}/{table}/rls")
		return
	}

	action := strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("action")))
	if action == "" {
		path := strings.TrimSuffix(request.URL.Path, "/")
		if strings.HasSuffix(path, "/enable") {
			action = "ENABLE"
		} else if strings.HasSuffix(path, "/disable") {
			action = "DISABLE"
		} else if strings.HasSuffix(path, "/force") {
			action = "FORCE"
		}
	}
	if action == "" && request.Body != nil {
		var toggleRLSInput ToggleRLSInput
		if decodeErr := json.NewDecoder(request.Body).Decode(&toggleRLSInput); decodeErr == nil {
			if toggleRLSInput.Action != "" {
				action = strings.ToUpper(strings.TrimSpace(toggleRLSInput.Action))
			} else if toggleRLSInput.Mode != "" {
				action = strings.ToUpper(strings.TrimSpace(toggleRLSInput.Mode))
			}
		}
	}

	switch action {
	case "ENABLE":
		if enableErr := controlPlaneHandler.ddlEngine.EnableRLS(request.Context(), schema, table); enableErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, enableErr.Error())
			return
		}
		if controlPlaneHandler.eventBus != nil {
			controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(schema+"."+table, TableUpdatedEventData{
				Schema: schema,
				Table:  table,
				Action: "toggle_rls",
				Detail: "enabled",
			}))
		}
		controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ToggleRLSResponse{
			Status: "enabled",
			Mode:   "ENABLE",
		})
	case "DISABLE":
		if disableErr := controlPlaneHandler.ddlEngine.DisableRLS(request.Context(), schema, table); disableErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, disableErr.Error())
			return
		}
		if controlPlaneHandler.eventBus != nil {
			controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(schema+"."+table, TableUpdatedEventData{
				Schema: schema,
				Table:  table,
				Action: "toggle_rls",
				Detail: "disabled",
			}))
		}
		controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ToggleRLSResponse{
			Status: "disabled",
			Mode:   "DISABLE",
		})
	case "FORCE":
		force := request.URL.Query().Get("force") != "false"
		if forceErr := controlPlaneHandler.ddlEngine.ForceRLS(request.Context(), schema, table, force); forceErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, forceErr.Error())
			return
		}
		status := "forced"
		if !force {
			status = "unforced"
		}
		if controlPlaneHandler.eventBus != nil {
			controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(schema+"."+table, TableUpdatedEventData{
				Schema: schema,
				Table:  table,
				Action: "toggle_rls",
				Detail: status,
			}))
		}
		controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ToggleRLSResponse{
			Status: status,
			Mode:   "FORCE",
		})
	case "UNFORCE":
		if unforceErr := controlPlaneHandler.ddlEngine.ForceRLS(request.Context(), schema, table, false); unforceErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, unforceErr.Error())
			return
		}
		if controlPlaneHandler.eventBus != nil {
			controlPlaneHandler.eventBus.Publish(request.Context(), NewTableUpdatedEvent(schema+"."+table, TableUpdatedEventData{
				Schema: schema,
				Table:  table,
				Action: "toggle_rls",
				Detail: "unforced",
			}))
		}
		controlPlaneHandler.writeJSON(responseWriter, http.StatusOK, ToggleRLSResponse{
			Status: "unforced",
			Mode:   "UNFORCE",
		})
	default:
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Unknown RLS action")
	}
}
