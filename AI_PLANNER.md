# AI Planner Service Integration

The aiPlanner service has been successfully integrated into your Places API. This service uses OpenRouter API to provide AI-powered travel planning capabilities.

## Configuration

### Environment Variables

Add the following environment variable to enable the AI planner service:

```bash
export PLACES_API_AI_OPENROUTER_API_KEY="sk-or-v1-your-openrouter-api-key-here"
```

Optional configuration:

```bash
export PLACES_API_AI_MODEL="deepseek/deepseek-chat-v3.1:free"  # Default model (free tier)
```

### YAML Configuration

The configuration has been added to `configs/config.yaml`:

```yaml
ai:
  openrouter_api_key: "" # Set via PLACES_API_AI_OPENROUTER_API_KEY environment variable
  model: "deepseek/deepseek-chat-v3.1:free" # Can override via PLACES_API_AI_MODEL environment variable
```

## API Endpoints

### 1. Generate Travel Plan

**POST** `/v1/ai/plan`

Generate an AI-powered travel plan for a specific area.

**Request Body:**

```json
{
  "area": "Rome, Italy",
  "duration": 3,
  "interests": ["history", "food", "art"],
  "budget": "medium",
  "travel_style": "cultural"
}
```

**Response:**

```json
{
  "id": "plan_uuid",
  "area": "Rome, Italy",
  "duration": 3,
  "title": "3-Day Cultural Journey in Rome",
  "description": "Explore Rome's rich history, art, and cuisine...",
  "days": [
    {
      "day": 1,
      "title": "Ancient Rome Discovery",
      "description": "Start your Roman adventure...",
      "activities": [
        {
          "time": "09:00",
          "title": "Colosseum Tour",
          "description": "Explore the iconic amphitheater...",
          "location": "Colosseum",
          "category": "sightseeing",
          "duration": "2 hours",
          "cost": "€16"
        }
      ]
    }
  ],
  "tips": ["Book Colosseum tickets in advance", "Try local trattorias"],
  "generated_at": "2023-09-16T10:00:00Z"
}
```

### 2. Travel Chat

**POST** `/v1/ai/chat`

Get AI-powered travel advice and recommendations.

**Request Body:**

```json
{
  "area": "Rome, Italy",
  "question": "What are the best local dishes to try?",
  "context": "I'm vegetarian and prefer authentic local experiences"
}
```

**Response:**

```json
{
  "answer": "For vegetarian options in Rome, I highly recommend...",
  "suggestions": [
    "Where can I find the best gelato?",
    "What's the best time to visit the Vatican?",
    "Are there any food tours for vegetarians?"
  ],
  "generated_at": "2023-09-16T10:00:00Z"
}
```

## Getting Started

1. **Get OpenRouter API Key:**

   - Sign up at [OpenRouter.ai](https://openrouter.ai/)
   - Create an API key
   - Add it to your environment variables

2. **Set Environment Variable:**

   ```bash
   export PLACES_API_AI_OPENROUTER_API_KEY="sk-or-v1-your-openrouter-api-key-here"
   ```

3. **Start the Server:**

   ```bash
   go run . server
   ```

4. **Test the Endpoints:**

   ```bash
   # Test plan generation
   curl -X POST http://localhost:8080/v1/ai/plan \
     -H "Content-Type: application/json" \
     -d '{
       "area": "Paris, France",
       "duration": 2,
       "interests": ["museums", "cafes"],
       "budget": "medium"
     }'

   # Test travel chat
   curl -X POST http://localhost:8080/v1/ai/chat \
     -H "Content-Type: application/json" \
     -d '{
       "area": "Paris, France",
       "question": "What are the best neighborhoods to stay in?",
       "context": "First time visitor, interested in art and food"
     }'
   ```

## Error Handling

- If the OpenRouter API key is not configured, the AI endpoints will return a 503 Service Unavailable error
- The service gracefully handles API failures and provides appropriate error messages
- All other API endpoints continue to work normally even if the AI service is unavailable

## Supported Models

The service defaults to `deepseek/deepseek-chat-v3.1:free` (free tier), but you can configure any OpenRouter-supported model:

- `deepseek/deepseek-chat-v3.1:free` (default, free tier)
- `anthropic/claude-3.5-sonnet` (excellent for travel planning)
- `openai/gpt-4-turbo`
- `openai/gpt-3.5-turbo`
- `google/gemini-pro`
- And many more available on OpenRouter

## Architecture

The AI planner service follows the same patterns as other services in the application:

1. **Service Layer** (`internal/services/aiplanner.go`):

   - Handles OpenRouter API integration
   - Manages request/response formatting
   - Provides error handling and logging

2. **Types** (`internal/types/types.go`):

   - Defines request/response structures
   - Ensures type safety across the application

3. **Handlers** (`internal/handlers/handlers.go`):

   - Provides HTTP endpoints
   - Validates requests
   - Returns formatted responses

4. **Configuration** (`internal/config/config.go`):
   - Manages environment variables
   - Provides configuration validation

The integration is complete and ready for production use!
