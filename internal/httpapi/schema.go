package httpapi

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type schemaCacheKey struct {
	pool *pgxpool.Pool
	name string
}

var verifiedSchemas sync.Map

func checkSchemaCached(ctx context.Context, db querier, name, query string) (bool, error) {
	pool, isPool := db.(*pgxpool.Pool)
	if isPool {
		if v, ok := verifiedSchemas.Load(schemaCacheKey{pool: pool, name: name}); ok && v.(bool) {
			return true, nil
		}
	}
	var ok bool
	e := db.QueryRow(ctx, query).Scan(&ok)
	if e == nil && ok && isPool {
		verifiedSchemas.Store(schemaCacheKey{pool: pool, name: name}, true)
	}
	return ok, e
}

func hasSemproSchema(ctx context.Context, db querier) (bool, error) {
	return checkSchemaCached(ctx, db, "sempro", `SELECT to_regclass('capstone_service.project_documents') IS NOT NULL AND to_regclass('capstone_service.proposal_seminars') IS NOT NULL AND to_regclass('capstone_service.seminar_evaluations') IS NOT NULL`)
}

func hasFullSOPSchema(ctx context.Context, db querier) (bool, error) {
	return checkSchemaCached(ctx, db, "full_sop", `SELECT to_regclass('capstone_service.academic_periods') IS NOT NULL AND to_regclass('capstone_service.capstone_groups') IS NOT NULL AND to_regclass('capstone_service.final_exams') IS NOT NULL AND to_regclass('capstone_service.expo_submissions') IS NOT NULL AND to_regclass('capstone_service.knowledge_sources') IS NOT NULL`)
}

func scheduleConflict(ctx context.Context, db querier, table, excludeProjectID string, at time.Time, location string, examiners []string) (string, error) {
	winStart := at.Add(-89 * time.Minute)
	winEnd := at.Add(89 * time.Minute)
	var conflictTitle string
	q := `SELECT p.title FROM capstone_service.` + table + ` s
		JOIN capstone_service.projects p ON p.id=s.project_id
		WHERE s.project_id::text <> $1
		  AND s.status IN ('SCHEDULED','REVISION')
		  AND s.scheduled_at BETWEEN $2 AND $3
		  AND (lower(s.location)=lower($4) OR s.examiner_1_id=ANY($5::uuid[]) OR s.examiner_2_id=ANY($5::uuid[]) OR s.examiner_3_id=ANY($5::uuid[]))
		LIMIT 1`
	rows, e := db.Query(ctx, q, excludeProjectID, winStart, winEnd, location, examiners)
	if e != nil {
		return "", e
	}
	defer rows.Close()
	if rows.Next() {
		if e = rows.Scan(&conflictTitle); e != nil {
			return "", e
		}
		return conflictTitle, nil
	}
	return "", rows.Err()
}
