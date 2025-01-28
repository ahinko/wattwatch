package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"
	"wattwatch/internal/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAuditLogRepository_Create(t *testing.T) {
	tc := testutil.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	metadata := map[string]interface{}{"key": "value"}
	metadataJSON, err := json.Marshal(metadata)
	require.NoError(t, err)

	tests := []struct {
		name    string
		log     *models.CreateAuditLogRequest
		wantErr bool
	}{
		{
			name: "Success",
			log: &models.CreateAuditLogRequest{
				UserID:      &user.ID,
				Action:      models.AuditActionCreate,
				EntityType:  "user",
				EntityID:    user.ID.String(),
				Description: "Created new user",
				Metadata:    string(metadataJSON),
				IPAddress:   "127.0.0.1",
				UserAgent:   "test-agent",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.AuditRepo.Create(context.Background(), tt.log)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Verify log was created
				var count int
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT COUNT(*) FROM audit_logs WHERE user_id = $1 AND action = $2",
					tt.log.UserID, tt.log.Action).Scan(&count)
				require.NoError(t, err)
				require.Equal(t, 1, count)
			}
		})
	}
}

func TestAuditLogRepository_GetByID(t *testing.T) {
	tc := testutil.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	metadata := map[string]interface{}{"key": "value"}
	metadataJSON, err := json.Marshal(metadata)
	require.NoError(t, err)

	// Create an audit log
	log := &models.CreateAuditLogRequest{
		UserID:      &user.ID,
		Action:      models.AuditActionCreate,
		EntityType:  "user",
		EntityID:    user.ID.String(),
		Description: "Created new user",
		Metadata:    string(metadataJSON),
		IPAddress:   "127.0.0.1",
		UserAgent:   "test-agent",
	}
	err = tc.AuditRepo.Create(context.Background(), log)
	require.NoError(t, err)

	// Get the created log's ID
	var logID uuid.UUID
	err = tc.DB.QueryRowContext(context.Background(),
		"SELECT id FROM audit_logs WHERE user_id = $1 AND action = $2",
		log.UserID, log.Action).Scan(&logID)
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
		errType error
	}{
		{
			name:    "Success",
			id:      logID,
			wantErr: false,
		},
		{
			name:    "Non-existent ID",
			id:      uuid.New(),
			wantErr: true,
			errType: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tc.AuditRepo.GetByID(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, log.UserID, result.UserID)
				require.Equal(t, log.Action, result.Action)
				require.Equal(t, log.EntityType, result.EntityType)
				require.Equal(t, log.EntityID, result.EntityID)
				require.Equal(t, log.Description, result.Description)
				require.Equal(t, log.IPAddress, result.IPAddress)
				require.Equal(t, log.UserAgent, result.UserAgent)
				require.False(t, result.CreatedAt.IsZero())
			}
		})
	}
}

func TestAuditLogRepository_List(t *testing.T) {
	tc := testutil.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create some audit logs with different characteristics
	startTime := time.Now().UTC().Add(-1 * time.Hour)
	logs := []*models.CreateAuditLogRequest{
		{
			UserID:      &user.ID,
			Action:      models.AuditActionCreate,
			EntityType:  "user",
			EntityID:    user.ID.String(),
			Description: "Created user",
			IPAddress:   "127.0.0.1",
			UserAgent:   "test-agent-1",
		},
		{
			UserID:      &user.ID,
			Action:      models.AuditActionUpdate,
			EntityType:  "user",
			EntityID:    user.ID.String(),
			Description: "Updated user settings",
			IPAddress:   "127.0.0.2",
			UserAgent:   "test-agent-2",
		},
		{
			UserID:      &user.ID,
			Action:      models.AuditActionDelete,
			EntityType:  "post",
			EntityID:    uuid.New().String(),
			Description: "Deleted post",
			IPAddress:   "127.0.0.3",
			UserAgent:   "test-agent-3",
		},
	}

	for _, log := range logs {
		err := tc.AuditRepo.Create(context.Background(), log)
		require.NoError(t, err)
	}

	endTime := time.Now().UTC().Add(1 * time.Hour)
	searchTerm := "user"
	ipAddress := "127.0.0.1"
	limit := 2
	offset := 1

	tests := []struct {
		name      string
		filter    repository.AuditLogFilter
		wantCount int
		wantErr   bool
	}{
		{
			name: "Filter by User ID",
			filter: repository.AuditLogFilter{
				UserID: &user.ID,
			},
			wantCount: 3,
		},
		{
			name: "Filter by Actions",
			filter: repository.AuditLogFilter{
				Actions: []models.AuditAction{models.AuditActionCreate, models.AuditActionUpdate},
			},
			wantCount: 2,
		},
		{
			name: "Filter by Entity Type",
			filter: repository.AuditLogFilter{
				EntityTypes: []string{"user"},
			},
			wantCount: 2,
		},
		{
			name: "Filter by IP Address",
			filter: repository.AuditLogFilter{
				IPAddress: &ipAddress,
			},
			wantCount: 1,
		},
		{
			name: "Filter by Date Range",
			filter: repository.AuditLogFilter{
				CreatedAfter:  &startTime,
				CreatedBefore: &endTime,
			},
			wantCount: 3,
		},
		{
			name: "Search Term",
			filter: repository.AuditLogFilter{
				SearchTerm: &searchTerm,
			},
			wantCount: 2,
		},
		{
			name: "With Pagination",
			filter: repository.AuditLogFilter{
				Limit:  &limit,
				Offset: &offset,
			},
			wantCount: 2,
		},
		{
			name: "Order By Created At DESC",
			filter: repository.AuditLogFilter{
				OrderBy:   "created_at",
				OrderDesc: true,
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := tc.AuditRepo.List(context.Background(), tt.filter)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, results, tt.wantCount)
			}
		})
	}
}

func TestAuditLogRepository_GetByUserID(t *testing.T) {
	tc := testutil.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create some audit logs
	logs := []*models.CreateAuditLogRequest{
		{
			UserID:      &user.ID,
			Action:      models.AuditActionCreate,
			EntityType:  "user",
			EntityID:    user.ID.String(),
			Description: "Created user",
			IPAddress:   "127.0.0.1",
			UserAgent:   "test-agent-1",
		},
		{
			UserID:      &user.ID,
			Action:      models.AuditActionUpdate,
			EntityType:  "user",
			EntityID:    user.ID.String(),
			Description: "Updated user",
			IPAddress:   "127.0.0.2",
			UserAgent:   "test-agent-2",
		},
	}

	for _, log := range logs {
		err := tc.AuditRepo.Create(context.Background(), log)
		require.NoError(t, err)
	}

	tests := []struct {
		name      string
		userID    uuid.UUID
		wantCount int
		wantErr   bool
	}{
		{
			name:      "Success",
			userID:    user.ID,
			wantCount: 2,
		},
		{
			name:      "Non-existent User",
			userID:    uuid.New(),
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := tc.AuditRepo.GetByUserID(context.Background(), tt.userID, repository.AuditLogFilter{})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, results, tt.wantCount)
				if tt.wantCount > 0 {
					for _, result := range results {
						require.Equal(t, tt.userID, *result.UserID)
					}
				}
			}
		})
	}
}

func TestAuditLogRepository_GetByEntityTypeAndID(t *testing.T) {
	tc := testutil.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)
	entityID := uuid.New().String()

	// Create some audit logs
	logs := []*models.CreateAuditLogRequest{
		{
			UserID:      &user.ID,
			Action:      models.AuditActionCreate,
			EntityType:  "post",
			EntityID:    entityID,
			Description: "Created post",
			IPAddress:   "127.0.0.1",
			UserAgent:   "test-agent-1",
		},
		{
			UserID:      &user.ID,
			Action:      models.AuditActionUpdate,
			EntityType:  "post",
			EntityID:    entityID,
			Description: "Updated post",
			IPAddress:   "127.0.0.2",
			UserAgent:   "test-agent-2",
		},
	}

	for _, log := range logs {
		err := tc.AuditRepo.Create(context.Background(), log)
		require.NoError(t, err)
	}

	tests := []struct {
		name       string
		entityType string
		entityID   string
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "Success",
			entityType: "post",
			entityID:   entityID,
			wantCount:  2,
		},
		{
			name:       "Non-existent Entity Type",
			entityType: "nonexistent",
			entityID:   entityID,
			wantCount:  0,
		},
		{
			name:       "Non-existent Entity ID",
			entityType: "post",
			entityID:   uuid.New().String(),
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := tc.AuditRepo.GetByEntityTypeAndID(context.Background(), tt.entityType, tt.entityID, repository.AuditLogFilter{})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, results, tt.wantCount)
				if tt.wantCount > 0 {
					for _, result := range results {
						require.Equal(t, tt.entityType, result.EntityType)
						require.Equal(t, tt.entityID, result.EntityID)
					}
				}
			}
		})
	}
}

func TestAuditLogRepository_CleanupOld(t *testing.T) {
	tc := testutil.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create some audit logs with different timestamps
	now := time.Now()
	oldLog := &models.CreateAuditLogRequest{
		UserID:      &user.ID,
		Action:      models.AuditActionCreate,
		EntityType:  "user",
		EntityID:    user.ID.String(),
		Description: "Old log",
		IPAddress:   "127.0.0.1",
		UserAgent:   "test-agent",
	}
	err := tc.AuditRepo.Create(context.Background(), oldLog)
	require.NoError(t, err)

	// Set the created_at to an old date
	_, err = tc.DB.ExecContext(context.Background(),
		"UPDATE audit_logs SET created_at = $1 WHERE description = $2",
		now.Add(-30*24*time.Hour), "Old log")
	require.NoError(t, err)

	// Create a recent log
	recentLog := &models.CreateAuditLogRequest{
		UserID:      &user.ID,
		Action:      models.AuditActionCreate,
		EntityType:  "user",
		EntityID:    user.ID.String(),
		Description: "Recent log",
		IPAddress:   "127.0.0.1",
		UserAgent:   "test-agent",
	}
	err = tc.AuditRepo.Create(context.Background(), recentLog)
	require.NoError(t, err)

	// Clean up logs older than 7 days
	err = tc.AuditRepo.CleanupOld(context.Background(), 7*24*time.Hour)
	require.NoError(t, err)

	// Verify only recent log remains
	var count int
	err = tc.DB.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM audit_logs").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Verify the remaining log is the recent one
	var description string
	err = tc.DB.QueryRowContext(context.Background(), "SELECT description FROM audit_logs").Scan(&description)
	require.NoError(t, err)
	require.Equal(t, "Recent log", description)
}
