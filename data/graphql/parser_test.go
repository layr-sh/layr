package graphql

import (
	"fmt"
	"testing"
)

func TestGraphqlParserParseAndValidateUnit(t *testing.T) {
	// 1. Complex Query with variables, nested relations, aliases, and arguments
	query := `
		query GetUserPosts($userId: String, $limit: Int) {
			user: users(where: { id: { _eq: $userId }, status: { _in: ["active", "pending"] }, archived: { _is_null: true } }, limit: $limit) {
				id
				name
				posts(where: { score: { _gte: 10, _lte: 100, _neq: 50 } }, limit: 5) {
					id
					title
					comments {
						id
						body
					}
				}
			}
		}
	`
	variables := map[string]any{
		"userId": "018f4a12-5678-7890-abcd-ef0123456789",
		"limit":  10,
	}

	operationNode, parseErr := ParseGraphQL(query, variables)
	if parseErr != nil {
		t.Fatalf("unexpected error parsing valid GraphQL: %v", parseErr)
	}

	if operationNode.Type != QueryOperationType || operationNode.Name != "GetUserPosts" {
		t.Fatalf("unexpected operation type or name: %+v", operationNode)
	}

	if len(operationNode.SelectionSet) != 1 {
		t.Fatalf("expected 1 root field, got %d", len(operationNode.SelectionSet))
	}

	fieldNode := operationNode.SelectionSet[0]
	if fieldNode.Name != "users" || fieldNode.Alias != "user" {
		t.Fatalf("unexpected root name/alias: %+v", fieldNode)
	}

	depth := operationNode.CalculateDepth()
	if depth != 4 { // users -> posts -> comments -> (id, body)
		t.Fatalf("expected depth 4, got %d", depth)
	}

	complexity := operationNode.CalculateComplexity()
	if complexity <= 0 {
		t.Fatalf("expected positive complexity, got %d", complexity)
	}

	// 1b. Test @cache directive parsing on query operation
	cacheDirQuery := `
		query GetCachedPosts($limit: Int) @cache(ttl: 60, key_suffix: "custom_posts") {
			posts(limit: $limit) {
				id
				title
			}
		}
	`
	cacheOperationNode, cacheErr := ParseGraphQL(cacheDirQuery, map[string]any{"limit": 5})
	if cacheErr != nil {
		t.Fatalf("unexpected error parsing cached query: %v", cacheErr)
	}
	if cacheOperationNode.CacheTTL != 60 || cacheOperationNode.CacheKeySuffix != "custom_posts" {
		t.Fatalf("expected CacheTTL=60 and CacheKeySuffix=custom_posts, got ttl=%d key=%s", cacheOperationNode.CacheTTL, cacheOperationNode.CacheKeySuffix)
	}

	cacheLegacyQuery := `query @cache(ttl: 20, key: "legacy_posts") { posts { id } }`
	cacheLegacyOperationNode, cacheLegacyErr := ParseGraphQL(cacheLegacyQuery, nil)
	if cacheLegacyErr != nil || cacheLegacyOperationNode.CacheKeySuffix != "legacy_posts" {
		t.Fatalf("expected CacheKeySuffix=legacy_posts on key fallback, got %v", cacheLegacyOperationNode)
	}

	// 1c. Test @cache with float64 ttl and int variable ttl
	cacheFloatQuery := `query @cache(ttl: 30.5) { users { id } }`
	cacheFloatOperationNode, cacheFloatErr := ParseGraphQL(cacheFloatQuery, nil)
	if cacheFloatErr != nil || cacheFloatOperationNode.CacheTTL != 30 {
		t.Fatalf("expected CacheTTL=30, got %d, err=%v", cacheFloatOperationNode.CacheTTL, cacheFloatErr)
	}

	cacheIntQuery := `query @cache(ttl: $t) { users { id } }`
	cacheIntOperationNode, cacheIntErr := ParseGraphQL(cacheIntQuery, map[string]any{"t": int(45)})
	if cacheIntErr != nil || cacheIntOperationNode.CacheTTL != 45 {
		t.Fatalf("expected CacheTTL=45, got %d, err=%v", cacheIntOperationNode.CacheTTL, cacheIntErr)
	}

	// Directive error
	if _, directiveErr := ParseGraphQL(`query @cache(!) { users { id } }`, nil); directiveErr == nil {
		t.Fatal("expected error parsing invalid directive args")
	}

	// 2. Mutations
	mutationDocument := `
		mutation CreateUser($userObject: Object) {
			insert_users(objects: [$userObject, { name: "Direct", is_verified: false, rating: 4.5, age: -20 }]) {
				id
				name
			}
			update_users(where: { id: { _eq: "123" } }, _set: { name: "New" }) {
				id
			}
			delete_users(where: { id: { _eq: "123" } }) {
				id
			}
		}
	`
	mutationVariables := map[string]any{
		"userObject": map[string]any{"name": "VarUser"},
	}

	mutationOperationNode, mutationErr := ParseGraphQL(mutationDocument, mutationVariables)
	if mutationErr != nil {
		t.Fatalf("unexpected error parsing mutation: %v", mutationErr)
	}
	if mutationOperationNode.Type != MutationOperationType || len(mutationOperationNode.SelectionSet) != 3 {
		t.Fatalf("unexpected mutation node: %+v", mutationOperationNode)
	}

	// 3. Anonymous query with comments, escaped string literals, enums, booleans, arrays, objects
	anonymousQuery := `
		# Leading comment
		{
			# Comment before field
			users(name: "John \"The Great\" Doe", unsupplied: $notProvided, emptyValue: null, active: true, inactive: false, role: MEMBER, list: [1, 2], config: { k1: 1, k2: "two" }) {
				id
			}
		}
	`
	anonymousOperationNode, anonErr := ParseGraphQL(anonymousQuery, nil)
	if anonErr != nil || anonymousOperationNode.Type != QueryOperationType || len(anonymousOperationNode.SelectionSet) != 1 {
		t.Fatalf("unexpected anon query: %+v, err: %v", anonymousOperationNode, anonErr)
	}

	// 4. Parser error branches
	errQueries := []string{
		"",                                // Empty
		"query",                           // Incomplete
		"query {",                         // Unclosed
		"query { users",                   // Missing closing
		"query { :users }",                // Bad alias
		"query { users: }",                // Missing field after alias
		"query { users( ) }",              // Empty args with closing
		"query { users(: 1) }",            // Missing arg name
		"query { users(a 1) }",            // Missing colon after arg name
		"query { users(where ) }",         // Missing colon
		"query { users(where: { ) }",      // Unclosed object
		"query { users(where: { a } )",    // Missing colon in object
		"query { users(where: [ ) }",      // Unclosed array
		`query { users(str: "unclosed) }`, // Unclosed string
		"query { users(where: { a: \n }) }",
		"query { users(a: !) }",           // Unexpected char
		"query { users(a: { k: 1 }",       // Missing closing brace
		"query { users(a: [ 1, 2 }",       // Missing closing bracket
		"query { ! }",                     // Expected field name
		"query { users(123) }",            // Expected arg name
		"query { users { posts { ! } } }", // Bad sub-field
		"query { users(a: { k: ! }) }",    // Bad object value
		"query { users(a: [ ! ]) }",       // Bad array value
		"query { users { id",              // Missing closing brace on selection set
		"query { users( a: 1",             // Missing closing paren on args
		"query { users() }",               // Empty args list
	}

	for queryIndex, errQuery := range errQueries {
		t.Run(fmt.Sprintf("ErrQuery_%d", queryIndex), func(t *testing.T) {
			if _, parseErr := ParseGraphQL(errQuery, nil); parseErr == nil {
				t.Fatalf("case %d: expected error for %q, got nil", queryIndex, errQuery)
			}
		})
	}

	// 5. Direct parser methods error testing
	parser := &Parser{input: []rune("not_a_brace"), length: len("not_a_brace")}
	if _, selectionErr := parser.parseSelectionSet(nil); selectionErr == nil {
		t.Fatal("expected error parsing selection set without '{'")
	}

	secondParser := &Parser{input: []rune("not_a_paren"), length: len("not_a_paren")}
	if _, argErr := secondParser.parseArguments(nil); argErr == nil {
		t.Fatal("expected error parsing arguments without '('")
	}

	braceParser := &Parser{input: []rune("{ a: { k 1 } }"), length: len("{ a: { k 1 } }")}
	_, _ = braceParser.parseSelectionSet(nil)

	parenParser := &Parser{input: []rune("not_a_paren"), length: len("not_a_paren")}
	if _, parenErr := parenParser.parseArguments(nil); parenErr == nil {
		t.Fatal("expected error parsing arguments without '('")
	}

	objectBadParser := &Parser{input: []rune("{ a: { k 1 } }"), length: len("{ a: { k 1 } }")}
	_, _ = objectBadParser.parseSelectionSet(nil)

	arrayBadParser := &Parser{input: []rune("{ a: [ ! ] }"), length: len("{ a: [ ! ] }")}
	_, _ = arrayBadParser.parseSelectionSet(nil)

	argNameBadParser := &Parser{input: []rune("( !: 1 )"), length: len("( !: 1 )")}
	_, _ = argNameBadParser.parseArguments(nil)

	argColonBadParser := &Parser{input: []rune("( a 1 )"), length: len("( a 1 )")}
	_, _ = argColonBadParser.parseArguments(nil)

	invalidValueParser := &Parser{input: []rune("!"), length: 1}
	_, _ = invalidValueParser.parseValue(nil)

	// 6. Test comma-separated args, object, array, and unsupplied var
	valueQuery := `query { users(a: 1, b: 2, c: { x: 1, y: 2 }, d: [1, 2], e: $missing_var, f: {}, g: [], h: false, i: null) { id } }`
	valueOperationNode, err := ParseGraphQL(valueQuery, nil)
	if err != nil || len(valueOperationNode.SelectionSet) != 1 {
		t.Fatalf("expected successful parse of literal values: %v", err)
	}

	unclosedSelectionParser := &Parser{input: []rune("{ id"), length: 4}
	_, _ = unclosedSelectionParser.parseSelectionSet(nil)

	unclosedArgumentsParser := &Parser{input: []rune("( a: 1"), length: 6}
	_, _ = unclosedArgumentsParser.parseArguments(nil)

	emptyArgumentsParser := &Parser{input: []rune("()"), length: 2}
	_, _ = emptyArgumentsParser.parseArguments(nil)

	unclosedObjectParser := &Parser{input: []rune("{ a: 1"), length: 6}
	_, _ = unclosedObjectParser.parseValue(nil)

	unclosedArrayParser := &Parser{input: []rune("[ 1, 2"), length: 6}
	_, _ = unclosedArrayParser.parseValue(nil)

	// 7. readNumber error branches
	invalidFloatParser := &Parser{input: []rune("1.2.3.4"), length: 7}
	if _, floatErr := invalidFloatParser.readNumber(); floatErr == nil {
		t.Fatal("expected error parsing invalid float 1.2.3.4")
	}

	invalidIntParser := &Parser{input: []rune("99999999999999999999999999999999999999999999999"), length: 47}
	if _, intErr := invalidIntParser.readNumber(); intErr == nil {
		t.Fatal("expected error parsing overflowing int")
	}

	// 8. String escape sequences (\n, \t, \r, \b, \f, \", \\, \/)
	escapedQuery := `query { users(text: "line1\nline2\ttab\rcarriage\bback\fform\"quote\\slash\/forward") { id } }`
	escapedOperationNode, escapedErr := ParseGraphQL(escapedQuery, nil)
	if escapedErr != nil {
		t.Fatalf("unexpected error parsing escaped string: %v", escapedErr)
	}
	expectedText := "line1\nline2\ttab\rcarriage\bback\fform\"quote\\slash/forward"
	if escapedOperationNode.SelectionSet[0].Arguments["text"] != expectedText {
		t.Fatalf("expected %q, got %q", expectedText, escapedOperationNode.SelectionSet[0].Arguments["text"])
	}

	// 8b. String with unrecognized escape rune falls back to literal rune
	unrecognizedEscapeOperationNode, unrecogErr := ParseGraphQL(`{ users(text: "hello\world") { id } }`, nil)
	if unrecogErr != nil || unrecognizedEscapeOperationNode.SelectionSet[0].Arguments["text"] != "helloworld" {
		t.Fatalf("expected helloworld, got %v (err: %v)", unrecognizedEscapeOperationNode.SelectionSet[0].Arguments["text"], unrecogErr)
	}

	// 9. Subscription rejection
	if _, subErr := ParseGraphQL(`subscription { users { id } }`, nil); subErr == nil {
		t.Fatal("expected error parsing subscription")
	}

	// 10. Multi-operation document with operation selection
	multiDoc := `
		query GetUsers { users { id } }
		query GetPosts { posts { id } }
	`
	postsOperationNode, postsErr := ParseGraphQLWithOperation(multiDoc, "GetPosts", nil)
	if postsErr != nil || postsOperationNode.Name != "GetPosts" {
		t.Fatalf("expected GetPosts operation, got %+v, err: %v", postsOperationNode, postsErr)
	}

	usersOperationNode, usersErr := ParseGraphQLWithOperation(multiDoc, "GetUsers", nil)
	if usersErr != nil || usersOperationNode.Name != "GetUsers" {
		t.Fatalf("expected GetUsers operation, got %+v, err: %v", usersOperationNode, usersErr)
	}

	if _, unknownOpErr := ParseGraphQLWithOperation(multiDoc, "UnknownOp", nil); unknownOpErr == nil {
		t.Fatal("expected error selecting unknown operation")
	}

	// 11. Balanced parentheses with strings in variable definition
	parenQuery := `query MyQuery($status: String = "default (paren) value") { users { id } }`
	parenthesesOperationNode, parenErr := ParseGraphQL(parenQuery, nil)
	if parenErr != nil || parenthesesOperationNode.Name != "MyQuery" {
		t.Fatalf("expected successful parse with paren in default string, got %+v, err: %v", parenthesesOperationNode, parenErr)
	}
}
