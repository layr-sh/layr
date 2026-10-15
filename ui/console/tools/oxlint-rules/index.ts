import enforceErrorNaming from "./enforce-error-naming.ts";
import enforceInitialismCasing from "./enforce-initialism-casing.ts";
import noDestructuredCallReturn from "./no-destructured-call-return.ts";
import noDirectCallAccess from "./no-direct-call-access.ts";
import noDisallowedIdentifiers from "./no-disallowed-identifiers.ts";
import noRepresentationSuffixes from "./no-representation-suffixes.ts";
import noShortIdentifiers from "./no-short-identifiers.ts";
import representTypeName from "./represent-type-name.ts";

const plugin = {
  meta: {
    name: "layr-oxlint-rules",
  },
  rules: {
    "no-short-identifiers": noShortIdentifiers,
    "no-representation-suffixes": noRepresentationSuffixes,
    "enforce-initialism-casing": enforceInitialismCasing,
    "no-disallowed-identifiers": noDisallowedIdentifiers,
    "enforce-error-naming": enforceErrorNaming,
    "represent-type-name": representTypeName,
    "no-direct-call-access": noDirectCallAccess,
    "no-destructured-call-return": noDestructuredCallReturn,
  },
};

export default plugin;
