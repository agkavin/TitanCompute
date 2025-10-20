"""
GGUF Quantization Configuration and Model Selection
Implements complete bartowski GGUF quantization support with memory-aware selection
"""

import logging
import psutil
from typing import Dict, List, Optional, Tuple
from dataclasses import dataclass
from enum import Enum


class QuantizationTier(Enum):
    """Quantization quality tiers based on available system memory"""
    PREMIUM = "premium"      # 8GB+ RAM: Q8_0, Q6_K_L, Q6_K
    HIGH = "high"           # 6-8GB RAM: Q5_K_M, Q4_K_M, Q4_K_S
    GOOD = "good"           # 4-6GB RAM: IQ4_XS, Q3_K_L, IQ3_M
    EMERGENCY = "emergency"  # <4GB RAM: Q2_K, IQ2_M


@dataclass
class QuantizationConfig:
    """Configuration for a specific quantization format"""
    name: str
    memory_overhead_mb: int
    quality_score: float  # 0-1, higher is better
    description: str
    arm_optimized: bool = False


class GGUFQuantizationManager:
    """Manages GGUF quantization selection and model optimization"""
    
    # Complete bartowski GGUF quantization configurations
    QUANTIZATIONS = {
        # Premium Quality (8GB+ RAM)
        "Q8_0": QuantizationConfig("Q8_0", 512, 0.95, "8-bit quantization, near original quality"),
        "Q6_K_L": QuantizationConfig("Q6_K_L", 384, 0.90, "6-bit mixed precision, large model"),
        "Q6_K": QuantizationConfig("Q6_K", 320, 0.88, "6-bit mixed precision"),
        
        # High Quality (6-8GB RAM)
        "Q5_K_M": QuantizationConfig("Q5_K_M", 256, 0.85, "5-bit mixed precision, medium"),
        "Q4_K_M": QuantizationConfig("Q4_K_M", 192, 0.80, "4-bit mixed precision, medium (default)"),
        "Q4_K_S": QuantizationConfig("Q4_K_S", 160, 0.78, "4-bit mixed precision, small"),
        
        # Good Quality (4-6GB RAM)
        "IQ4_XS": QuantizationConfig("IQ4_XS", 128, 0.75, "4-bit improved quantization, extra small"),
        "Q3_K_L": QuantizationConfig("Q3_K_L", 112, 0.70, "3-bit mixed precision, large"),
        "IQ3_M": QuantizationConfig("IQ3_M", 96, 0.68, "3-bit improved quantization, medium"),
        
        # Emergency Fallback (<4GB RAM)
        "Q2_K": QuantizationConfig("Q2_K", 64, 0.60, "2-bit quantization, minimal quality"),
        "IQ2_M": QuantizationConfig("IQ2_M", 48, 0.55, "2-bit improved quantization, minimal"),
        
        # ARM CPU Optimizations
        "Q4_0_4_4": QuantizationConfig("Q4_0_4_4", 144, 0.76, "4-bit ARM optimization", arm_optimized=True),
        "Q4_0_8_8": QuantizationConfig("Q4_0_8_8", 160, 0.78, "4-bit ARM optimization, larger", arm_optimized=True),
    }
    
    # Memory tier thresholds in MB
    MEMORY_TIERS = {
        QuantizationTier.PREMIUM: 8192,    # 8GB+
        QuantizationTier.HIGH: 6144,       # 6GB+
        QuantizationTier.GOOD: 4096,       # 4GB+
        QuantizationTier.EMERGENCY: 0,     # Any
    }
    
    # Quantizations by tier (ordered by quality)
    TIER_QUANTIZATIONS = {
        QuantizationTier.PREMIUM: ["Q8_0", "Q6_K_L", "Q6_K"],
        QuantizationTier.HIGH: ["Q5_K_M", "Q4_K_M", "Q4_K_S"],
        QuantizationTier.GOOD: ["IQ4_XS", "Q3_K_L", "IQ3_M"],
        QuantizationTier.EMERGENCY: ["Q2_K", "IQ2_M"],
    }

    def __init__(self):
        self.logger = logging.getLogger(__name__)
        self.is_arm = self._detect_arm_architecture()
        
    def _detect_arm_architecture(self) -> bool:
        """Detect if running on ARM architecture"""
        try:
            import platform
            arch = platform.machine().lower()
            return any(arm in arch for arm in ['arm', 'aarch64', 'arm64'])
        except Exception:
            return False
    
    def get_system_memory_info(self) -> Tuple[int, int]:
        """Get total and available system memory in MB"""
        try:
            memory = psutil.virtual_memory()
            total_mb = memory.total // (1024 * 1024)
            available_mb = memory.available // (1024 * 1024)
            return total_mb, available_mb
        except Exception as e:
            self.logger.warning(f"Failed to get memory info: {e}")
            return 4096, 2048  # Conservative defaults
    
    def determine_optimal_tier(self, available_memory_mb: int) -> QuantizationTier:
        """Determine the optimal quantization tier based on available memory"""
        for tier, threshold in self.MEMORY_TIERS.items():
            if available_memory_mb >= threshold:
                return tier
        return QuantizationTier.EMERGENCY
    
    def get_recommended_quantization(
        self, 
        model_base: str,
        available_memory_mb: Optional[int] = None,
        prefer_quality: bool = True
    ) -> str:
        """Get the recommended quantization for a model"""
        if available_memory_mb is None:
            _, available_memory_mb = self.get_system_memory_info()
        
        # Determine tier
        tier = self.determine_optimal_tier(available_memory_mb)
        
        # Get quantizations for tier
        quantizations = self.TIER_QUANTIZATIONS[tier].copy()
        
        # Add ARM optimizations if applicable
        if self.is_arm and tier in [QuantizationTier.HIGH, QuantizationTier.GOOD]:
            quantizations.extend(["Q4_0_4_4", "Q4_0_8_8"])
        
        # Sort by quality if preferred, otherwise by memory efficiency
        if prefer_quality:
            quantizations.sort(key=lambda q: self.QUANTIZATIONS[q].quality_score, reverse=True)
        else:
            quantizations.sort(key=lambda q: self.QUANTIZATIONS[q].memory_overhead_mb)
        
        selected = quantizations[0]
        
        self.logger.info(f"Selected quantization {selected} for tier {tier.value} "
                        f"(available memory: {available_memory_mb}MB)")
        
        return selected
    
    def build_model_name(
        self, 
        base_model: str, 
        quantization: Optional[str] = None,
        available_memory_mb: Optional[int] = None
    ) -> str:
        """Build the complete model name with quantization"""
        if quantization is None:
            quantization = self.get_recommended_quantization(base_model, available_memory_mb)
        
        # Handle bartowski convention: model-name-quantization
        if ":" in base_model:
            # Already has tag, replace with quantization
            base = base_model.split(":")[0]
            return f"{base}:{quantization.lower()}"
        elif base_model.endswith("-GGUF"):
            # Bartowski format: Meta-Llama-3.1-8B-Instruct-GGUF
            return f"{base_model}:{quantization}"
        else:
            # Standard format
            return f"{base_model}:{quantization.lower()}"
    
    def get_quantization_info(self, quantization: str) -> Optional[QuantizationConfig]:
        """Get configuration for a specific quantization"""
        return self.QUANTIZATIONS.get(quantization.upper())
    
    def list_available_quantizations(
        self, 
        available_memory_mb: Optional[int] = None
    ) -> Dict[QuantizationTier, List[str]]:
        """List all available quantizations organized by tier"""
        if available_memory_mb is None:
            _, available_memory_mb = self.get_system_memory_info()
        
        result = {}
        current_tier = self.determine_optimal_tier(available_memory_mb)
        
        for tier in QuantizationTier:
            if tier.value <= current_tier.value or tier == QuantizationTier.EMERGENCY:
                result[tier] = self.TIER_QUANTIZATIONS[tier].copy()
                
                # Add ARM optimizations if applicable
                if self.is_arm and tier in [QuantizationTier.HIGH, QuantizationTier.GOOD]:
                    result[tier].extend(["Q4_0_4_4", "Q4_0_8_8"])
        
        return result
    
    def estimate_model_memory_usage(
        self, 
        model_name: str, 
        quantization: str,
        base_model_size_mb: Optional[int] = None
    ) -> int:
        """Estimate total memory usage for a quantized model"""
        quant_config = self.get_quantization_info(quantization)
        if not quant_config:
            return 4096  # Conservative default
        
        if base_model_size_mb is None:
            # Rough estimation based on model name patterns
            if "1b" in model_name.lower() or "1B" in model_name:
                base_model_size_mb = 2048
            elif "7b" in model_name.lower() or "7B" in model_name:
                base_model_size_mb = 6144
            elif "13b" in model_name.lower() or "13B" in model_name:
                base_model_size_mb = 10240
            else:
                base_model_size_mb = 4096  # Default assumption
        
        # Calculate quantized size based on quality score (inverse relationship)
        quantized_size = int(base_model_size_mb * (1.0 - quant_config.quality_score + 0.2))
        total_usage = quantized_size + quant_config.memory_overhead_mb
        
        return total_usage
