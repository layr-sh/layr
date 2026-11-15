
# CreateTableInput

CreateTableInput schema

## Properties

Name | Type
------------ | -------------
`columns` | [Array&lt;Column&gt;](Column.md)
`foreignKeys` | [Array&lt;ForeignKey&gt;](ForeignKey.md)
`name` | string
`primaryKeyName` | string
`schema` | string

## Example

```typescript
import type { CreateTableInput } from ''

// TODO: Update the object below with actual values
const example = {
  "columns": null,
  "foreignKeys": null,
  "name": null,
  "primaryKeyName": null,
  "schema": null,
} satisfies CreateTableInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as CreateTableInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


