import psutil
import time
import logging
from dataclasses import dataclass
from typing import Optional

try:
    import pynvml
    HAS_GPU = True
    pynvml.nvmlInit()
except (ImportError, pynvml.NVMLError):
    HAS_GPU = False
    logging.warning("NVIDIA GPU monitoring not available")


@dataclass
class SystemStats:
    """System resource statistics"""
    free_vram_mb: int
    total_vram_mb: int
    free_ram_mb: int
    total_ram_mb: int
    cpu_percent: float
    gpu_temperature: Optional[int] = None


class StatsCollector:
    """Collects system resource statistics"""

    def __init__(self):
        self.has_gpu = HAS_GPU
        self.gpu_handle = None
        
        if self.has_gpu:
            try:
                # Get first GPU device
                self.gpu_handle = pynvml.nvmlDeviceGetHandleByIndex(0)
                logging.info("GPU monitoring initialized")
            except pynvml.NVMLError as e:
                logging.warning(f"Failed to initialize GPU monitoring: {e}")
                self.has_gpu = False

    def collect(self) -> SystemStats:
        """Collect current system statistics"""
        # Memory stats
        memory = psutil.virtual_memory()
        free_ram_mb = memory.available // (1024 * 1024)
        total_ram_mb = memory.total // (1024 * 1024)
        
        # CPU stats
        cpu_percent = psutil.cpu_percent(interval=0.1)
        
        # GPU stats
        free_vram_mb = 0
        total_vram_mb = 0
        gpu_temperature = None
        
        if self.has_gpu and self.gpu_handle:
            try:
                mem_info = pynvml.nvmlDeviceGetMemoryInfo(self.gpu_handle)
                total_vram_mb = mem_info.total // (1024 * 1024)
                free_vram_mb = mem_info.free // (1024 * 1024)
                
                # Get GPU temperature
                gpu_temperature = pynvml.nvmlDeviceGetTemperature(
                    self.gpu_handle, pynvml.NVML_TEMPERATURE_GPU
                )
            except pynvml.NVMLError as e:
                logging.warning(f"Failed to collect GPU stats: {e}")
        
        return SystemStats(
            free_vram_mb=free_vram_mb,
            total_vram_mb=total_vram_mb,
            free_ram_mb=free_ram_mb,
            total_ram_mb=total_ram_mb,
            cpu_percent=cpu_percent,
            gpu_temperature=gpu_temperature
        )
