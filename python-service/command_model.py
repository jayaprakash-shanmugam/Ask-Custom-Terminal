# This file should contain your CommandModelTrainer class from the original code
# I'm including it here for completeness, but you would use your existing code

import json
import numpy as np
import pandas as pd
from sklearn.model_selection import train_test_split
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.pipeline import Pipeline
from sklearn.svm import SVC
import pickle
import os
import re
from collections import defaultdict, Counter

class CommandModelTrainer:
    def __init__(self, model_path=None):
        """
        Initialize the CommandModelTrainer with an optional pre-trained model
        
        Args:
            model_path: Path to a saved model file
        """
        self.model = None
        self.vectorizer = None
        self.bigrams = defaultdict(Counter)

        if model_path and os.path.exists(model_path):
            self.load_model(model_path)
        else:
            # Initialize a new model
            self.vectorizer = TfidfVectorizer(ngram_range=(1, 3))
            self.model = SVC(kernel='linear', probability=True)
    
    def load_dataset(self, dataset_path):
        """
        Load the command dataset from a JSON file
        
        Args:
            dataset_path: Path to the JSON file containing command data
            
        Returns:
            pandas.DataFrame: DataFrame containing the command data
        """
        with open(dataset_path, 'r') as f:
            data = json.load(f)
        
        # Convert to DataFrame
        df = pd.DataFrame(data)
        
        # Create a command_string column that combines command and args
        df['command_with_args'] = df.apply(
            lambda row: {'command': row['command'], 'args': row['args']}, 
            axis=1
        )
        
        return df
    
    def preprocess_text(self, text):
        """
        Preprocess text for the model
        
        Args:
            text: Input text string
            
        Returns:
            str: Preprocessed text
        """
        # Convert to lowercase
        text = text.lower()
        
        # Replace special characters with spaces
        text = re.sub(r'[^\w\s]', ' ', text)
        
        # Remove extra whitespace
        text = re.sub(r'\s+', ' ', text).strip()
        
        return text
    
    def train(self, dataset_path, model_save_path=None, test_size=0.2):
        """
        Train the model on the command dataset
        
        Args:
            dataset_path: Path to the JSON file containing command data
            model_save_path: Path to save the trained model
            test_size: Proportion of data to use for testing
            
        Returns:
            float: Accuracy score of the model on the test set
        """
        # Load dataset
        df = self.load_dataset(dataset_path)
        
        # Preprocess natural language descriptions
        df['processed_nl'] = df['natural_language'].apply(self.preprocess_text)
        
        # Create dictionary mapping
        command_map = {}
        for i, row in df.iterrows():
            key = f"{row['command']}_{','.join(row['args'])}"
            command_map[key] = {'command': row['command'], 'args': row['args'], 'explanation': row['explanation']}
        
        # Save this mapping for prediction phase
        if model_save_path:
            mapping_path = f"{os.path.splitext(model_save_path)[0]}_mapping.json"
            os.makedirs(os.path.dirname(mapping_path), exist_ok=True)
            with open(mapping_path, 'w') as f:
                json.dump(command_map, f, indent=2)
        
        # Split data
        X_train, X_test, y_train, y_test = train_test_split(
            df['processed_nl'], 
            df.apply(lambda row: f"{row['command']}_{','.join(row['args'])}", axis=1),
            test_size=test_size, 
            random_state=42
        )
        
        # Create pipeline
        pipeline = Pipeline([
            ('vectorizer', self.vectorizer),
            ('classifier', self.model)
        ])
        
        # Train model
        pipeline.fit(X_train, y_train)
        
        # Evaluate
        accuracy = pipeline.score(X_test, y_test)
        
        # Store the trained pipeline for future use
        self.model = pipeline.named_steps['classifier']
        self.vectorizer = pipeline.named_steps['vectorizer']

        # Save model if a path is provided
        if model_save_path:
            os.makedirs(os.path.dirname(model_save_path), exist_ok=True)
            self.save_model(model_save_path)
        
        return accuracy
    
    def predict(self, query, mapping_path=None):
        """
        Predict the command for a natural language query
        
        Args:
            query: Natural language query
            mapping_path: Path to the command mapping file
            
        Returns:
            dict: Predicted command, args, explanation and confidence
        """
        if not self.model or not self.vectorizer:
            raise ValueError("Model not trained or loaded")
        
        # Load mapping
        command_map = {}
        if mapping_path and os.path.exists(mapping_path):
            with open(mapping_path, 'r') as f:
                command_map = json.load(f)
        
        # Preprocess query
        processed_query = self.preprocess_text(query)
        
        # Transform query to vector
        query_vector = self.vectorizer.transform([processed_query])
        
        # Get prediction and probability
        prediction = self.model.predict(query_vector)[0]
        probabilities = self.model.predict_proba(query_vector)[0]
        confidence = max(probabilities)
        print(f"Prediction: {prediction}, Confidence: {confidence}")
        if not confidence:
            return{
                'command': None,
                'args': None,
                'explanation': "Low confidence in prediction",
                'confidence': float(confidence)
            }
        
        # Get command details from mapping
        if prediction in command_map:
            cmd_details = command_map[prediction]
            return {
                'command': cmd_details['command'],
                'args': cmd_details['args'],
                'explanation': cmd_details['explanation'],
                'confidence': float(confidence)
            }
        else:
            # Parse the prediction string
            parts = prediction.split('_')
            command = parts[0]
            args = parts[1].split(',') if len(parts) > 1 and parts[1] else []
            
            return {
                'command': command,
                'args': args,
                'explanation': "Generated based on your request",
                'confidence': float(confidence)
            }
        
    def build_ngram_model(self, dataset_path):
        """
        Build a bigram model from the natural language commands
        
        Args:
            dataset_path: Path to the JSON file containing command data
        """

        # Load dataset
        df = self.load_dataset(dataset_path)

        self.bigrams = defaultdict(Counter)
        for text in df['natural_language']:
            tokens = self.tokenize(text)
            for i in range(len(tokens) - 1):
                current_word = tokens[i]
                next_word = tokens[i + 1]
                # Update bigram counts
                self.bigrams[current_word][next_word] += 1

        # Save bigram model as part of the overall model state
        if hasattr(self, 'model_path') and self.model_path:
            self.save_model(self.model_path)
        
    def predict_next_word(self, query, top_k=3):
        """
        Predict the next word based on the last word using bigram model

        Args:
            query: Partial natural language string
            top_k: Number of suggestions to return

        Returns:
            list: Top-k predicted next words
        """

        # Ensure the bigram model is built
        if not self.bigrams:
            return []

        tokens = self.tokenize(query)
        if not tokens:
            return []
        last_word = tokens[-1]
        next_word_counter = self.bigrams.get(last_word)
        if not next_word_counter:
            return []
        return [word for word, _ in next_word_counter.most_common(top_k)]

    def save_model(self, model_path):
        """
        Save the trained model to a file
        
        Args:
            model_path: Path to save the model
        """
        self.model_path = model_path
        model_data = {
            'vectorizer': self.vectorizer,
            'model': self.model,
            'bigrams': dict(self.bigrams)
        }
        
        with open(model_path, 'wb') as f:
            pickle.dump(model_data, f)
    
    def load_model(self, model_path):
        """
        Load a trained model from a file
        
        Args:
            model_path: Path to the saved model
        """
        self.model_path = model_path
        with open(model_path, 'rb') as f:
            model_data = pickle.load(f)
        
        self.vectorizer = model_data['vectorizer']
        self.model = model_data['model']

        # Load bigrams if available, otherwise initialize empty
        if 'bigrams' in model_data:
            # Convert back to defaultdict(Counter)
            self.bigrams = defaultdict(Counter)
            for word, counter_dict in model_data['bigrams'].items():
                self.bigrams[word] = Counter(counter_dict)
        else:
            self.bigrams = defaultdict(Counter)

    def tokenize(self, text):
        """
        Tokenize text into words
        
        Args:
            text: Input text string
            
        Returns:
            list: List of tokens/words
        """
        # Convert to lowercase and extract word tokens
        return re.findall(r'\w+', text.lower())
