package app

import "github.com/jonathanselby/garage-crm/internal/sqlite"

// CoolingDays is the default age threshold for a quiet relationship or deal.
const CoolingDays = 14

// CoolingItems identifies deals and prospects without an open next task.
// It reads existing records only: no scheduler, notifications or persistence.
func (s *Service) CoolingItems(brand string) ([]sqlite.Row, error) {
	oppBrand, personBrand := "", ""
	args := []any{}
	if brand != "" {
		oppBrand = " AND o.brand_id=?"
		personBrand = " AND bp.brand_id=?"
		args = append(args, brand, brand)
	}
	args = append(args, CoolingDays)

	// julianday handles RFC3339 UTC timestamps (including fractional seconds).
	// Person activity is scoped to the business relationship, so a conversation
	// for one brand does not reset a prospect in another brand.
	query := `SELECT entity,id,label,brand_id,detail,days_since,COUNT(*) OVER() AS total_count
FROM (
 SELECT 'opportunities' entity,o.id,o.title label,o.brand_id,
        COALESCE(p.name,'') detail,
        CAST(julianday('now')-MAX(julianday(o.updated_at),
          COALESCE((SELECT MAX(julianday(a.occurred_at)) FROM activities a
                    WHERE a.opportunity_id=o.id),julianday(o.updated_at)))
            AS INTEGER) days_since
 FROM opportunities o
 JOIN brands b ON b.id=o.brand_id AND b.archived=0
 LEFT JOIN people p ON p.id=o.person_id
 WHERE o.archived=0
   AND UPPER(TRIM(o.stage)) NOT IN ('WON','LOST','CLOSED','CLOSED WON','CLOSED LOST')
   AND NOT EXISTS (SELECT 1 FROM tasks t WHERE t.opportunity_id=o.id
                   AND t.archived=0 AND t.status!='Done')` + oppBrand + `
 UNION ALL
 SELECT 'people' entity,bp.person_id id,p.name label,bp.brand_id,
        bp.lifecycle detail,
        CAST(julianday('now')-MAX(julianday(p.created_at),
          COALESCE((SELECT MAX(julianday(a.occurred_at)) FROM activities a
                    WHERE a.person_id=bp.person_id AND a.brand_id=bp.brand_id),
                   julianday(p.created_at))) AS INTEGER) days_since
 FROM brand_people bp
 JOIN people p ON p.id=bp.person_id AND p.archived=0
 JOIN brands b ON b.id=bp.brand_id AND b.archived=0
 WHERE UPPER(TRIM(bp.lifecycle)) NOT IN
       ('CUSTOMER','CLIENT','WON','LOST','CLOSED','INACTIVE','ARCHIVED','DISQUALIFIED')
   AND NOT EXISTS (SELECT 1 FROM tasks t WHERE t.person_id=bp.person_id
                   AND (t.brand_id=bp.brand_id OR t.brand_id IS NULL)
                   AND t.archived=0 AND t.status!='Done')
   AND NOT EXISTS (SELECT 1 FROM opportunities o
                   WHERE o.person_id=bp.person_id AND o.brand_id=bp.brand_id
                     AND o.archived=0
                     AND UPPER(TRIM(o.stage)) NOT IN
                         ('WON','LOST','CLOSED','CLOSED WON','CLOSED LOST'))` + personBrand + `
) WHERE days_since >= ?
ORDER BY days_since DESC, entity, label LIMIT 50`
	return s.DB.Query(query, args...)
}
