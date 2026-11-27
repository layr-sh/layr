package data

import "testing"

func TestDataSQLTransactionValidationUnit(t *testing.T) {
	valid := []string{
		"SELECT 1; SELECT 2", "-- COMMIT\nSELECT 1", "-- COMMIT", "/* outer /* COMMIT */ end */ SELECT 1;",
		"SELECT 'COMMIT; ROLLBACK';", "SELECT 'it''s safe';", "SELECT \"COMMIT\";", `SELECT E'escaped\'quote;COMMIT';`, "SELECT e'\\\\';",
		"CREATE FUNCTION f() RETURNS text AS $$ BEGIN RETURN 'COMMIT;'; END $$ LANGUAGE plpgsql;",
		"DO $body_1$ BEGIN NULL; END $body_1$;", "DO $é$ BEGIN NULL; END $é$;", "SELECT $1", "SELECT x$tag$identifier", "SELECT 1+$bad-tag$", "SELECT date'2026-01-01'", "SELECT ';';\nSELECT 2",
	}
	for _, sql := range valid {
		t.Run("case", func(t *testing.T) {
			if err := validateSQLTransaction(sql); err != nil {
				t.Errorf("rejected valid script %q: %v", sql, err)
			}
		})
	}
	invalid := []string{"BEGIN", "COMMIT;", "SELECT 1; -- comment\n ROLLBACK", "/*comment*/ START TRANSACTION", "END", "ABORT", "SAVEPOINT point", "RELEASE point", "PREPARE TRANSACTION 'id'", "SELECT 1;/*nested /* hi */ */COMMIT", "SELECT 'unterminated", "SELECT E'\\", "DO $body$ COMMIT", "SELECT 1 /* unterminated", "SELECT type'backslash\\'; COMMIT; -- quote"}
	for _, sql := range invalid {
		t.Run("case", func(t *testing.T) {
			if err := validateSQLTransaction(sql); err == nil {
				t.Errorf("accepted unsafe or incomplete script %q", sql)
			}
		})
	}
	for _, tag := range []string{"", "tag", "_tag_12", "é_tag"} {
		t.Run("case", func(t *testing.T) {
			if !validSQLDollarTag(tag) {
				t.Errorf("invalid tag %q", tag)
			}
		})
	}
	for _, tag := range []string{"1tag", "bad-tag", "a!"} {
		t.Run("case", func(t *testing.T) {
			if validSQLDollarTag(tag) {
				t.Errorf("accepted tag %q", tag)
			}
		})
	}
	for _, value := range []byte{'a', 'Z', '0', '_', '$', 200} {
		t.Run("case", func(t *testing.T) {
			if !sqlIdentifierByte(value) {
				t.Errorf("expected identifier byte %d", value)
			}
		})
	}
	if sqlIdentifierByte(' ') {
		t.Fatal("space is not an identifier")
	}
}
