
# Table


## Properties

Name | Type
------------ | -------------
`columns` | [Array&lt;Column&gt;](Column.md)
`estimatedRowCount` | number
`foreignKeys` | [Array&lt;ForeignKey&gt;](ForeignKey.md)
`indexes` | [Array&lt;ModelIndex&gt;](ModelIndex.md)
`name` | string
`primaryKey` | string
`rlsEnabled` | boolean
`rlsForced` | boolean
`schema` | string
`sizeBytes` | number

## Example

```typescript
import type { Table } from ''

// TODO: Update the object below with actual values
const example = {
  "columns": null,
  "estimatedRowCount": null,
  "foreignKeys": null,
  "indexes": null,
  "name": null,
  "primaryKey": null,
  "rlsEnabled": null,
  "rlsForced": null,
  "schema": null,
  "sizeBytes": null,
} satisfies Table

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Table
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


