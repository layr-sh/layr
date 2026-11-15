
# CreatePolicyInput

CreatePolicyInput schema

## Properties

Name | Type
------------ | -------------
`command` | string
`name` | string
`permissive` | string
`roles` | Array&lt;string&gt;
`usingExpression` | string
`withCheckExpression` | string

## Example

```typescript
import type { CreatePolicyInput } from ''

// TODO: Update the object below with actual values
const example = {
  "command": null,
  "name": null,
  "permissive": null,
  "roles": null,
  "usingExpression": null,
  "withCheckExpression": null,
} satisfies CreatePolicyInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as CreatePolicyInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


