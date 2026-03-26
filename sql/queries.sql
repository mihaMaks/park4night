-- name: CreateRecord :exec
INSERT INTO records (date, state, adults, children, bicycles)
VALUES (?, ?, ?, ?, ?);

-- name: ListRecords :many
SELECT id, date, state, adults, children, bicycles, created_at, updated_at
FROM records
ORDER BY date DESC, id DESC
LIMIT ? OFFSET ?;

-- name: ListRecordsWithFilters :many
SELECT id, date, state, adults, children, bicycles, created_at, updated_at
FROM records
WHERE
    (:date_filter IS NULL OR date LIKE :date_filter || '%') AND
    (:state_filter IS NULL OR state = :state_filter) AND
    (:min_adults IS NULL OR adults >= :min_adults) AND
    (:min_children IS NULL OR children >= :min_children) AND
    (:min_bicycles IS NULL OR bicycles >= :min_bicycles)
ORDER BY date DESC, id DESC
LIMIT :limit OFFSET :offset;

-- name: CountRecordsWithFilters :one
SELECT COUNT(*) as count
FROM records
WHERE
    (:date_filter IS NULL OR date LIKE :date_filter || '%') AND
    (:state_filter IS NULL OR state = :state_filter) AND
    (:min_adults IS NULL OR adults >= :min_adults) AND
    (:min_children IS NULL OR children >= :min_children) AND
    (:min_bicycles IS NULL OR bicycles >= :min_bicycles);

-- name: GetRecord :one
SELECT id, date, state, adults, children, bicycles, created_at, updated_at
FROM records
WHERE id = ?;

-- name: DeleteRecord :exec
DELETE FROM records
WHERE id = ?;

-- name: UpdateRecord :exec
UPDATE records
SET date = ?, state = ?, adults = ?, children = ?, bicycles = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: CountRecords :one
SELECT COUNT(*) as count
FROM records;

-- name: ListRecordsForChart :many
SELECT id, date, state, adults, children, bicycles
FROM records
WHERE
    (? IS NULL OR date >= ?) AND
    (? IS NULL OR date <= ?) AND
    (? IS NULL OR children >= ?) AND
    (? IS NULL OR bicycles >= ?)
ORDER BY date ASC;
