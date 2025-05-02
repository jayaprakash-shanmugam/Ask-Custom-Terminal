import json
import os
import time
import logging
from datetime import datetime, timezone
from flask import Flask, request, jsonify
from pymongo import MongoClient
from command_model import CommandModelTrainer

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler("server.log"),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger("terminal-model-server")

# Initialize Flask app
app = Flask(__name__)

# Initialize MongoDB client
mongo_uri = os.environ.get('MONGO_URI', 'mongodb://localhost:27017/')
db_name = os.environ.get('MONGO_DB', 'terminal_assistant')
try:
    mongo_client = MongoClient(mongo_uri)
    db = mongo_client[db_name]
    audit_collection = db['terminal_model_server_audit']
    logger.info(f"Connected to MongoDB at {mongo_uri}")
except Exception as e:
    logger.error(f"Failed to connect to MongoDB: {str(e)}")
    mongo_client = None
    db = None
    audit_collection = None

# Initialize model
MODEL_PATH = os.environ.get('MODEL_PATH', 'model/command_model.pkl')
MAPPING_PATH = os.environ.get('MAPPING_PATH', 'model/command_model_mapping.json')
DATA_PATH = os.environ.get('DATA_PATH', 'data/command_dataset.json')

# Create model directory if it doesn't exist
os.makedirs(os.path.dirname(MODEL_PATH), exist_ok=True)

# Initialize trainer
trainer = CommandModelTrainer(MODEL_PATH if os.path.exists(MODEL_PATH) else None)

def log_audit(event_type, message, metadata=None):
    """Log an audit event to MongoDB"""
    if audit_collection is None:
        logger.warning("MongoDB not available for audit logging")
        return
    
    audit_entry = {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "event_type": event_type,
        "message": message,
        "metadata": metadata or {}
    }
    
    try:
        audit_collection.insert_one(audit_entry)
    except Exception as e:
        logger.error(f"Failed to write audit log to MongoDB: {str(e)}")

@app.route('/ping', methods=['GET'])
def ping():
    """Simple ping endpoint to check if service is running"""
    return jsonify({
        "status": "ok",
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "service": "terminal-model-server"
    })

@app.route('/health', methods=['GET'])
def health_check():
    """Comprehensive health check endpoint"""
    health_data = {
        "status": "healthy",
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "service": "terminal-model-server",
        "model_loaded": trainer.model is not None,
        "checks": {
            "model": trainer.model is not None,
            "vectorizer": trainer.vectorizer is not None,
            "database": audit_collection is not None
        }
    }
    
    # Check if vectorizer is fitted
    if trainer.vectorizer is not None:
        try:
            # A simple check to see if the vectorizer has vocabulary_
            health_data["checks"]["vectorizer_fitted"] = hasattr(trainer.vectorizer, 'vocabulary_')
        except:
            health_data["checks"]["vectorizer_fitted"] = False
    
    # Check MongoDB connection
    if mongo_client is not None:
        try:
            # Check MongoDB connection by running a simple command
            mongo_client.admin.command('ping')
            health_data["checks"]["mongodb_connection"] = True
        except Exception as e:
            health_data["checks"]["mongodb_connection"] = False
            health_data["checks"]["mongodb_error"] = str(e)
    else:
        health_data["checks"]["mongodb_connection"] = False
    
    # Overall status depends on all critical checks
    if not all([
        health_data["checks"]["model"],
        health_data["checks"]["vectorizer"],
        health_data.get("checks", {}).get("vectorizer_fitted", False),
        health_data["checks"]["database"],
        health_data["checks"].get("mongodb_connection", False)
    ]):
        health_data["status"] = "degraded"
    
    # Log health check
    log_audit("health_check", "Health check performed", health_data)
    
    return jsonify(health_data)

@app.route('/predict', methods=['POST'])
def predict():
    """Predict command from natural language query"""
    start_time = time.time()
    
    # Get request data
    data = request.json
    if not data or 'query' not in data:
        logger.warning("Invalid request: missing query")
        log_audit("predict_error", "Missing query in request", {"error": "Missing query"})
        return jsonify({'error': 'Query is required'}), 400
    
    client_id = data.get('client_id', 'unknown')
    session_id = data.get('session_id', 'unknown')
    query = data['query']
    
    try:
        # Make prediction
        result = trainer.predict(query, MAPPING_PATH)
        
        # Add request ID and metadata
        result['request_id'] = str(int(time.time() * 1000))
        result['timestamp'] = datetime.now(timezone.utc).isoformat()
        
        # Calculate duration
        duration = time.time() - start_time
        result['duration'] = f"{duration:.4f}s"
        
        # Log the request and response
        log_audit("prediction", "Command predicted from query", {
            "client_id": client_id,
            "session_id": session_id,
            "query": query,
            "command": result['command'],
            "args": result['args'],
            "confidence": result['confidence'],
            "duration": duration,
            "request_id": result['request_id']
        })
        
        return jsonify(result)
    
    except Exception as e:
        error_message = str(e)
        logger.error(f"Error making prediction: {error_message}", exc_info=True)
        
        log_audit("predict_error", "Error making prediction", {
            "client_id": client_id,
            "session_id": session_id, 
            "query": query,
            "error": error_message
        })
        
        return jsonify({
            'error': error_message,
            'status': 'error',
            'request_id': str(int(time.time() * 1000))
        }), 500
    
@app.route('/predictNextWord', methods=['POST'])
def predict_next_word():
    """Predict the next word in a command"""
    start_time = time.time()
    
    # Get request data
    data = request.json
    if not data or 'query' not in data:
        logger.warning("Invalid request: missing query")
        log_audit("predict_next_word_error", "Missing query in request", {"error": "Missing query"})
        return jsonify({'error': 'Query is required'}), 400
    
    client_id = data.get('client_id', 'unknown')
    session_id = data.get('session_id', 'unknown')
    query = data['query']
    
    try:
        # Make prediction
        predicted_words = trainer.predict_next_word(query)

        # Create a dictionary with the result
        result = {
            'next_word_suggestions': predicted_words,
            'predicted_word': predicted_words[0] if predicted_words else '',
            'confidence': 1.0 if predicted_words else 0.0,  # Default confidence
            'request_id': str(int(time.time() * 1000)),
            'timestamp': datetime.now(timezone.utc).isoformat()
        }
        
        # Calculate duration
        duration = time.time() - start_time
        result['duration'] = f"{duration:.4f}s"
        
        # Log the request and response
        log_audit("next_word_prediction", "Next word predicted from query", {
            "client_id": client_id,
            "session_id": session_id,
            "query": query,
            "predicted_words": predicted_words,
            "duration": duration,
            "request_id": result['request_id']
        })
        
        return jsonify(result)
    
    except Exception as e:
        error_message = str(e)
        logger.error(f"Error making next word prediction: {error_message}", exc_info=True)
        
        log_audit("predict_next_word_error", "Error making next word prediction", {
            "client_id": client_id,
            "session_id": session_id, 
            "query": query,
            "error": error_message
        })
        
        return jsonify({
            'error': error_message,
            'status': 'error',
            'request_id': str(int(time.time() * 1000))
        }), 500

@app.route('/train', methods=['POST'])
def train_model():
    """Train or retrain the model"""
    try:
        # Get request data if any
        data = request.json if request.is_json else {}
        dataset_path = data.get('dataset_path', DATA_PATH)
        
        logger.info(f"Training model on dataset: {dataset_path}")
        log_audit("training_started", "Model training started", {"dataset_path": dataset_path})
        
        start_time = time.time()
        accuracy = trainer.train(dataset_path, MODEL_PATH)
        duration = time.time() - start_time
        
        logger.info(f"Model training completed with accuracy: {accuracy:.4f}")
        
        # Log completion
        log_audit("training_completed", "Model training completed", {
            "dataset_path": dataset_path,
            "accuracy": accuracy,
            "model_path": MODEL_PATH,
            "duration": f"{duration:.2f}s"
        })
        
        return jsonify({
            'status': 'success',
            'accuracy': accuracy,
            'model_path': MODEL_PATH,
            'duration': f"{duration:.2f}s"
        })
    
    except Exception as e:
        error_message = str(e)
        logger.error(f"Error training model: {error_message}", exc_info=True)
        
        log_audit("training_error", "Error during model training", {
            "dataset_path": data.get('dataset_path', DATA_PATH),
            "error": error_message
        })
        
        return jsonify({
            'error': error_message,
            'status': 'error'
        }), 500

@app.route('/get_audits', methods=['GET'])
def get_audits():
    """Get audit logs with filtering options"""
    try:
        # Parse query parameters
        limit = int(request.args.get('limit', 100))
        skip = int(request.args.get('skip', 0))
        event_type = request.args.get('event_type')
        
        # Build query
        query = {}
        if event_type:
            query["event_type"] = event_type
        
        # Get results from MongoDB
        if audit_collection is not None:
            cursor = audit_collection.find(query).sort("timestamp", -1).skip(skip).limit(limit)
            
            # Convert to list and format for JSON
            results = []
            for doc in cursor:
                doc["_id"] = str(doc["_id"])  # Convert ObjectId to string
                doc["timestamp"] = doc["timestamp"].isoformat()  # Format datetime
                results.append(doc)
                
            return jsonify({
                "status": "success",
                "count": len(results),
                "data": results
            })
        else:
            return jsonify({
                "status": "error",
                "error": "MongoDB not available"
            }), 503
    
    except Exception as e:
        return jsonify({
            "status": "error",
            "error": str(e)
        }), 500

if __name__ == '__main__':
    # Get port from environment or use default
    port = int(os.environ.get('PORT', 5000))
    
    # Check if model exists, otherwise train one
    if not os.path.exists(MODEL_PATH):
        logger.warning(f"No model found at {MODEL_PATH}. Training a new model...")
        if os.path.exists(DATA_PATH):
            try:
                trainer.train(DATA_PATH, MODEL_PATH)
                logger.info("Model training completed")
                log_audit("initial_training", "Initial model training at startup", {
                    "model_path": MODEL_PATH,
                    "dataset_path": DATA_PATH
                })

                # initialize n-gram model
                trainer.build_ngram_model(DATA_PATH)
                logger.info("N-gram model built successfully")
                log_audit("ngram_model_built", "N-gram model built successfully", {
                    "dataset_path": DATA_PATH
                })
            except Exception as e:
                logger.error(f"Error training model: {str(e)}", exc_info=True)
                logger.warning("You will need to train a model using the /train endpoint before using predictions")
        else:
            logger.warning(f"No dataset found at {DATA_PATH}. You will need to train a model before using predictions")
    
    logger.info(f"Starting terminal model server on port {port}")
    log_audit("service_start", "Terminal model server started", {"port": port})
    app.run(host='0.0.0.0', port=port)