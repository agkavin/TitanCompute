import httpx
import json
import logging
import asyncio
import psutil
from typing import AsyncIterator, Optional, Dict, Any, List
from .quantization import GGUFQuantizationManager, QuantizationTier


class ModelManager:
    """Manages Ollama model operations with GGUF quantization support"""

    def __init__(self, ollama_host: str = "http://localhost:11434"):
        self.ollama_host = ollama_host
        self.client = httpx.AsyncClient(
            base_url=ollama_host,
            timeout=httpx.Timeout(300.0, connect=10.0)
        )
        self.loaded_models = set()
        self.logger = logging.getLogger(__name__)
        self.quantization_manager = GGUFQuantizationManager()
        self.model_registry = {}  # Cache for model metadata

    async def preload_models(self, models: List[str]):
        """Preload models on agent startup with intelligent quantization selection"""
        for model in models:
            self.logger.info(f"Preloading model: {model}")
            try:
                # Select optimal model variant based on available memory
                optimal_model = await self.select_optimal_model_variant(model)
                await self.pull_model(optimal_model)
                self.loaded_models.add(optimal_model)
                self.logger.info(f"Successfully preloaded: {optimal_model}")
                
                # Cache the mapping from base model to optimized model
                self.model_registry[model] = optimal_model
                
            except Exception as e:
                self.logger.error(f"Failed to preload {model}: {e}")

    async def select_optimal_model_variant(self, base_model: str) -> str:
        """Select the optimal quantized variant of a model based on system memory"""
        try:
            # Get current memory status
            total_mb, available_mb = self.quantization_manager.get_system_memory_info()
            
            # Reserve some memory for system operations
            usable_memory = max(available_mb - 1024, available_mb * 0.8)
            
            self.logger.info(f"System memory: {total_mb}MB total, {available_mb}MB available, "
                           f"{usable_memory:.0f}MB usable for model")
            
            # Check if model already has quantization suffix
            if any(quant in base_model.upper() for quant in self.quantization_manager.QUANTIZATIONS.keys()):
                self.logger.info(f"Model {base_model} already has quantization, using as-is")
                return base_model
            
            # Build optimal model name with quantization
            optimal_model = self.quantization_manager.build_model_name(
                base_model, 
                available_memory_mb=int(usable_memory)
            )
            
            # Log quantization selection details
            quantization = optimal_model.split(":")[-1].upper()
            quant_info = self.quantization_manager.get_quantization_info(quantization)
            tier = self.quantization_manager.determine_optimal_tier(int(usable_memory))
            
            self.logger.info(f"Selected quantization: {quantization} "
                           f"(tier: {tier.value}, quality: {quant_info.quality_score:.2f})")
            
            return optimal_model
            
        except Exception as e:
            self.logger.warning(f"Failed to select optimal variant for {base_model}: {e}")
            return base_model  # Fallback to original model name

    async def get_memory_usage_estimate(self, model_name: str) -> int:
        """Estimate memory usage for a model in MB"""
        try:
            # Extract quantization from model name
            if ":" in model_name:
                quantization = model_name.split(":")[-1].upper()
            else:
                quantization = "Q4_K_M"  # Default assumption
                
            return self.quantization_manager.estimate_model_memory_usage(
                model_name, quantization
            )
        except Exception as e:
            self.logger.warning(f"Failed to estimate memory usage for {model_name}: {e}")
            return 4096  # Conservative default

    async def can_load_model(self, model_name: str) -> bool:
        """Check if there's enough memory to load a model"""
        try:
            _, available_mb = self.quantization_manager.get_system_memory_info()
            estimated_usage = await self.get_memory_usage_estimate(model_name)
            
            # Keep at least 1GB free for system operations
            can_load = (available_mb - estimated_usage) > 1024
            
            self.logger.debug(f"Memory check for {model_name}: "
                            f"estimated {estimated_usage}MB, available {available_mb}MB, "
                            f"can_load: {can_load}")
            
            return can_load
        except Exception as e:
            self.logger.warning(f"Memory check failed for {model_name}: {e}")
            return False  # Conservative default

    async def get_quantization_recommendations(self) -> Dict[str, Any]:
        """Get quantization recommendations for current system"""
        try:
            total_mb, available_mb = self.quantization_manager.get_system_memory_info()
            tier = self.quantization_manager.determine_optimal_tier(available_mb)
            quantizations = self.quantization_manager.list_available_quantizations(available_mb)
            
            return {
                "system_memory": {
                    "total_mb": total_mb,
                    "available_mb": available_mb,
                    "recommended_tier": tier.value
                },
                "available_quantizations": {
                    tier.value: quants for tier, quants in quantizations.items()
                },
                "is_arm_optimized": self.quantization_manager.is_arm
            }
        except Exception as e:
            self.logger.error(f"Failed to get quantization recommendations: {e}")
            return {}

    async def pull_model(self, model_name: str):
        """Download model if not present"""
        try:
            response = await self.client.post(
                "/api/pull",
                json={"name": model_name}
            )
            response.raise_for_status()
            
            async for line in response.aiter_lines():
                if line:
                    try:
                        data = json.loads(line)
                        if "status" in data:
                            self.logger.debug(f"Pull status: {data['status']}")
                        if data.get("error"):
                            raise Exception(data["error"])
                    except json.JSONDecodeError:
                        continue
                        
        except Exception as e:
            self.logger.error(f"Failed to pull model {model_name}: {e}")
            raise

    async def stream_inference(
        self,
        model: str,
        prompt: str,
        **options
    ) -> AsyncIterator[Dict[str, Any]]:
        """Stream inference tokens from Ollama with intelligent model selection"""
        
        # Check if we have an optimized variant for this model
        actual_model = self.model_registry.get(model, model)
        
        # Ensure model is loaded
        if actual_model not in self.loaded_models:
            self.logger.info(f"Model {actual_model} not preloaded, selecting optimal variant...")
            
            # Select optimal variant if not already optimized
            if actual_model == model:
                actual_model = await self.select_optimal_model_variant(model)
            
            # Check if we can load the model
            if not await self.can_load_model(actual_model):
                # Try to find a smaller quantization
                self.logger.warning(f"Insufficient memory for {actual_model}, trying smaller quantization")
                
                # Get available memory and select emergency quantization
                _, available_mb = self.quantization_manager.get_system_memory_info()
                emergency_model = self.quantization_manager.build_model_name(
                    model, 
                    quantization="Q2_K",  # Emergency fallback
                    available_memory_mb=available_mb
                )
                actual_model = emergency_model
            
            await self.pull_model(actual_model)
            self.loaded_models.add(actual_model)
            self.model_registry[model] = actual_model

        self.logger.debug(f"Using model {actual_model} for inference request {model}")

        payload = {
            "model": actual_model,
            "prompt": prompt,
            "stream": True,
            "options": options
        }

        try:
            async with self.client.stream(
                "POST",
                "/api/generate",
                json=payload
            ) as response:
                response.raise_for_status()
                
                async for line in response.aiter_lines():
                    if line:
                        try:
                            chunk = json.loads(line)
                            yield chunk
                        except json.JSONDecodeError as e:
                            self.logger.warning(f"Failed to parse JSON: {e}")
                            continue
                        
        except Exception as e:
            self.logger.error(f"Inference failed: {e}")
            raise

    async def get_model_info(self, model: str) -> Optional[Dict[str, Any]]:
        """Get model metadata"""
        try:
            response = await self.client.post(
                "/api/show",
                json={"name": model}
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            self.logger.warning(f"Failed to get model info for {model}: {e}")
            return None

    async def list_models(self) -> list[Dict[str, Any]]:
        """List available models"""
        try:
            response = await self.client.get("/api/tags")
            response.raise_for_status()
            data = response.json()
            return data.get("models", [])
        except Exception as e:
            self.logger.error(f"Failed to list models: {e}")
            return []

    async def get_system_status(self) -> Dict[str, Any]:
        """Get comprehensive system status including quantization info"""
        try:
            # Get memory info
            total_mb, available_mb = self.quantization_manager.get_system_memory_info()
            
            # Get loaded models with their actual variants
            model_info = []
            for base_model, actual_model in self.model_registry.items():
                model_info.append({
                    "base_model": base_model,
                    "loaded_model": actual_model,
                    "estimated_memory_mb": await self.get_memory_usage_estimate(actual_model)
                })
            
            # Get quantization recommendations
            recommendations = await self.get_quantization_recommendations()
            
            return {
                "memory": {
                    "total_mb": total_mb,
                    "available_mb": available_mb,
                    "usage_percent": ((total_mb - available_mb) / total_mb * 100) if total_mb > 0 else 0
                },
                "loaded_models": model_info,
                "quantization": recommendations,
                "system_info": {
                    "is_arm": self.quantization_manager.is_arm,
                    "loaded_model_count": len(self.loaded_models)
                }
            }
        except Exception as e:
            self.logger.error(f"Failed to get system status: {e}")
            return {}

    async def close(self):
        """Close the HTTP client"""
        await self.client.aclose()
