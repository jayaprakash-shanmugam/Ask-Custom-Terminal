// audit-service/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AuditEvent represents the structure of an audit event
type AuditEvent struct {
	ID        primitive.ObjectID     `bson:"_id,omitempty" json:"_id,omitempty"`
	EventType string                 `bson:"event_type" json:"event_type"`
	Message   string                 `bson:"message" json:"message"`
	Timestamp time.Time              `bson:"timestamp" json:"timestamp"`
	Metadata  map[string]interface{} `bson:"metadata" json:"metadata"`
}

// AuditResponse is the structure returned for paginated audit queries
type AuditResponse struct {
	Data     []AuditEvent `json:"data"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
}

var (
	clientCollection *mongo.Collection
	modelCollection  *mongo.Collection
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Connect to MongoDB
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	// Initialize collections
	database := client.Database("terminal_assistant")
	clientCollection = database.Collection("terminal_client_server_audit")
	modelCollection = database.Collection("terminal_model_server_audit")

	// Setup Gin router
	router := gin.Default()

	// Configure CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Define API routes
	apiRoutes := router.Group("/api")
	{
		// Health check endpoint
		apiRoutes.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "healthy"})
		})

		// Audit endpoints
		auditRoutes := apiRoutes.Group("/audits")
		{
			auditRoutes.GET("/client", getClientAudits)
			auditRoutes.GET("/model", getModelAudits)
			auditRoutes.GET("/client/:id", getClientAuditByID)
			auditRoutes.GET("/model/:id", getModelAuditByID)
			auditRoutes.GET("/session/:sessionId", getSessionAudits)
			auditRoutes.GET("/stats", getAuditStats)
		}
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on port %s", port)
	router.Run(":" + port)
}

// getClientAudits handles requests for paginated client audit events
func getClientAudits(c *gin.Context) {
	getAudits(c, clientCollection)
}

// getModelAudits handles requests for paginated model audit events
func getModelAudits(c *gin.Context) {
	getAudits(c, modelCollection)
}

// getAudits is a helper function to fetch paginated audit events from a collection
func getAudits(c *gin.Context, collection *mongo.Collection) {
	// Parse query parameters
	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if pageVal, err := parseInt(pageStr); err == nil && pageVal > 0 {
			page = pageVal
		}
	}

	pageSize := 10
	if pageSizeStr := c.Query("pageSize"); pageSizeStr != "" {
		if pageSizeVal, err := parseInt(pageSizeStr); err == nil && pageSizeVal > 0 && pageSizeVal <= 100 {
			pageSize = pageSizeVal
		}
	}

	// Build filter based on query parameters
	filter := bson.M{}

	if eventType := c.Query("eventType"); eventType != "" {
		filter["event_type"] = eventType
	}

	if clientID := c.Query("clientId"); clientID != "" {
		filter["metadata.client_id"] = clientID
	}

	if sessionID := c.Query("sessionId"); sessionID != "" {
		filter["metadata.session_id"] = sessionID
	}

	if startDateStr := c.Query("startDate"); startDateStr != "" {
		if startDate, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			if endDateStr := c.Query("endDate"); endDateStr != "" {
				if endDate, err := time.Parse(time.RFC3339, endDateStr); err == nil {
					filter["timestamp"] = bson.M{
						"$gte": startDate,
						"$lte": endDate,
					}
				}
			} else {
				filter["timestamp"] = bson.M{"$gte": startDate}
			}
		}
	} else if endDateStr := c.Query("endDate"); endDateStr != "" {
		if endDate, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			filter["timestamp"] = bson.M{"$lte": endDate}
		}
	}

	// Count total documents for pagination
	total, err := collection.CountDocuments(context.Background(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count documents"})
		return
	}

	// Set options for pagination and sorting
	findOptions := options.Find()
	findOptions.SetSort(bson.M{"timestamp": -1})
	findOptions.SetSkip(int64((page - 1) * pageSize))
	findOptions.SetLimit(int64(pageSize))

	// Execute query
	cursor, err := collection.Find(context.Background(), filter, findOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit events"})
		return
	}
	defer cursor.Close(context.Background())

	// Decode results
	var results []AuditEvent
	if err := cursor.All(context.Background(), &results); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode audit events"})
		return
	}

	// Return paginated response
	c.JSON(http.StatusOK, AuditResponse{
		Data:     results,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// getClientAuditByID fetches a single client audit event by ID
func getClientAuditByID(c *gin.Context) {
	getAuditByID(c, clientCollection)
}

// getModelAuditByID fetches a single model audit event by ID
func getModelAuditByID(c *gin.Context) {
	getAuditByID(c, modelCollection)
}

// getAuditByID is a helper function to fetch a single audit event by ID
func getAuditByID(c *gin.Context, collection *mongo.Collection) {
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var result AuditEvent
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Audit event not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit event"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// getSessionAudits fetches all audit events for a specific session
func getSessionAudits(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session ID is required"})
		return
	}

	// First fetch client audits
	clientFilter := bson.M{"metadata.session_id": sessionID}
	clientCursor, err := clientCollection.Find(context.Background(), clientFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve client audit events"})
		return
	}
	defer clientCursor.Close(context.Background())

	var clientResults []AuditEvent
	if err := clientCursor.All(context.Background(), &clientResults); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode client audit events"})
		return
	}

	// Then fetch model audits
	modelFilter := bson.M{"metadata.session_id": sessionID}
	modelCursor, err := modelCollection.Find(context.Background(), modelFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve model audit events"})
		return
	}
	defer modelCursor.Close(context.Background())

	var modelResults []AuditEvent
	if err := modelCursor.All(context.Background(), &modelResults); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode model audit events"})
		return
	}

	// Combine and sort all results
	allResults := append(clientResults, modelResults...)

	c.JSON(http.StatusOK, gin.H{
		"data":  allResults,
		"total": len(allResults),
	})
}

// getAuditStats provides statistics about audit events
func getAuditStats(c *gin.Context) {
	// Get count of client events by type
	clientPipeline := mongo.Pipeline{
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$event_type"},
				{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			}},
		},
	}

	clientCursor, err := clientCollection.Aggregate(context.Background(), clientPipeline)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get client statistics"})
		return
	}
	defer clientCursor.Close(context.Background())

	var clientStats []bson.M
	if err := clientCursor.All(context.Background(), &clientStats); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode client statistics"})
		return
	}

	// Get count of model events by type
	modelPipeline := mongo.Pipeline{
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$event_type"},
				{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			}},
		},
	}

	modelCursor, err := modelCollection.Aggregate(context.Background(), modelPipeline)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get model statistics"})
		return
	}
	defer modelCursor.Close(context.Background())

	var modelStats []bson.M
	if err := modelCursor.All(context.Background(), &modelStats); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode model statistics"})
		return
	}

	// Get total counts
	clientTotal, _ := clientCollection.CountDocuments(context.Background(), bson.M{})
	modelTotal, _ := modelCollection.CountDocuments(context.Background(), bson.M{})

	c.JSON(http.StatusOK, gin.H{
		"clientStats": clientStats,
		"modelStats":  modelStats,
		"clientTotal": clientTotal,
		"modelTotal":  modelTotal,
	})
}

// Helper function to safely parse integers
func parseInt(s string) (int, error) {
	var value int
	_, err := fmt.Sscanf(s, "%d", &value)
	return value, err
}
