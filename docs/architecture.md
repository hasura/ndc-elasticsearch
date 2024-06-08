# General Architecture of the Elasticsearch Connector

### Overview

The Elasticsearch Connector takes a `QueryRequest`, which contains information about the query a user would like to run, translates it to an Elasticsearch DSL (Domain Specific Language) query, executes it against Elasticsearch, and returns the results as a `QueryResponse`.

### Components

The connector has three main components:

1. **prepareElasticsearchQuery**
2. **Search**
3. **prepareResponse**

### Details

#### 1. prepareElasticsearchQuery

Transforms `QueryRequest` into Elasticsearch DSL (Domain Specific Language) queries.

**Functionality:**
- Parse the incoming `QueryRequest`.
- Construct the DSL query with parameters like source, size, from, sort, and query.

**API:**

```go
func prepareElasticsearchQuery(
  ctx context.Context, 
  request *schema.QueryRequest, 
  state *types.State
) (map[string]interface{}, error)
```

#### 2. Search

Executes the DSL query against Elasticsearch and returns results.

**Functionality:**
- Execute queries on the specified Elasticsearch index.
- Fetch search results.

**API:**

```go
func (e *Client) Search(
  ctx context.Context,
  index string, 
  body map[string]interface{}
) (map[string]interface{}, error)
```

#### 3. prepareResponse

Converts Elasticsearch search results into `QueryResponse`.

**Functionality:**
- Traverse each field in the Elasticsearch response.
- Construct and return the QueryResponse, ensuring it conforms to the expected fields.

**API:**

```go
func prepareResponse(
  ctx context.Context, 
  response map[string]interface{}
) *schema.RowSet
```
## Workflow

1. **Receive `QueryRequest`:**
   - Incoming `QueryRequest` are intercepted by the connector.

2. **Prepare Query:**
   - The `prepareElasticsearchQuery` component processes the incoming query, translating it into an Elasticsearch DSL query.

3. **Execute Query:**
   - The `Search` component executes the DSL query against Elasticsearch and retrieves the raw response.

4. **Prepare Response:**
   - The `prepareResponse` component transforms the raw Elasticsearch response into a `QueryResponse`.