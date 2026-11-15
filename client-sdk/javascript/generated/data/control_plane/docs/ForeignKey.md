
# ForeignKey


## Properties

Name | Type
------------ | -------------
`column` | string
`constraintName` | string
`foreignColumn` | string
`foreignSchema` | string
`foreignTable` | string
`onDelete` | string
`onUpdate` | string

## Example

```typescript
import type { ForeignKey } from ''

// TODO: Update the object below with actual values
const example = {
  "column": null,
  "constraintName": null,
  "foreignColumn": null,
  "foreignSchema": null,
  "foreignTable": null,
  "onDelete": null,
  "onUpdate": null,
} satisfies ForeignKey

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ForeignKey
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


