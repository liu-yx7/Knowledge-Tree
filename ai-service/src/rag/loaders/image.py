"""Image document loader for RAG indexing.

This module provides functionality to extract text descriptions from images
using vision models or OCR for vector embedding and retrieval.
"""

import base64
import io
import logging
from dataclasses import dataclass
from pathlib import Path

from src.config import get_settings

logger = logging.getLogger(__name__)


@dataclass
class ImageChunk:
    """A chunk of image-derived content for embedding."""
    
    text: str
    chunk_index: int
    image_description: str  # AI-generated description
    ocr_text: str | None  # OCR-extracted text if available


class ImageLoader:
    """Loader for image documents that handles captioning and OCR.
    
    Uses OpenAI's vision model for image captioning. Can optionally
    use OCR for text extraction from images.
    """

    def __init__(
        self,
        chunk_size: int | None = None,
        chunk_overlap: int | None = None,
        use_ocr: bool = False,
    ):
        """Initialize the image loader.
        
        Args:
            chunk_size: Maximum characters per chunk. Defaults to config value.
            chunk_overlap: Overlap between chunks. Defaults to config value.
            use_ocr: Whether to attempt OCR text extraction.
        """
        settings = get_settings()
        self.chunk_size = chunk_size or settings.rag_chunk_size
        self.chunk_overlap = chunk_overlap or settings.rag_chunk_overlap
        self.use_ocr = use_ocr
        self._openai_client = None

    @property
    def openai_client(self):
        """Lazy-load OpenAI client."""
        if self._openai_client is None:
            try:
                from openai import OpenAI
                settings = get_settings()
                self._openai_client = OpenAI(
                    api_key=settings.openai_api_key,
                    base_url=settings.openai_base_url,
                )
            except ImportError:
                logger.error("openai not installed. Run: pip install openai")
                raise
        return self._openai_client

    def load_from_path(self, file_path: str | Path) -> list[ImageChunk]:
        """Load and process an image from a file path.
        
        Args:
            file_path: Path to the image file.
            
        Returns:
            List of ImageChunk objects ready for embedding.
        """
        try:
            with open(file_path, 'rb') as f:
                image_bytes = f.read()
            
            # Detect MIME type
            mime_type = self._detect_mime_type(file_path)
            return self.load_from_bytes(image_bytes, mime_type)
            
        except Exception as e:
            logger.error(f"Failed to load image from {file_path}: {e}")
            return []

    def load_from_bytes(
        self,
        image_bytes: bytes,
        mime_type: str = "image/jpeg",
    ) -> list[ImageChunk]:
        """Load and process an image from bytes.
        
        Args:
            image_bytes: Raw image content.
            mime_type: MIME type of the image.
            
        Returns:
            List of ImageChunk objects ready for embedding.
        """
        try:
            # Generate image description using vision model
            description = self._generate_description(image_bytes, mime_type)
            
            # Optionally extract OCR text
            ocr_text = None
            if self.use_ocr:
                ocr_text = self._extract_ocr_text(image_bytes)
            
            # Combine description and OCR text
            combined_text = self._combine_text(description, ocr_text)
            
            if not combined_text:
                logger.warning("No text extracted from image")
                return []
            
            # Create chunk(s)
            return self._create_chunks(combined_text, description, ocr_text)
            
        except Exception as e:
            logger.error(f"Failed to process image: {e}")
            return []

    def _detect_mime_type(self, file_path: str | Path) -> str:
        """Detect MIME type from file extension.
        
        Args:
            file_path: Path to the file.
            
        Returns:
            MIME type string.
        """
        path = Path(file_path)
        extension = path.suffix.lower()
        
        mime_types = {
            '.jpg': 'image/jpeg',
            '.jpeg': 'image/jpeg',
            '.png': 'image/png',
            '.gif': 'image/gif',
            '.webp': 'image/webp',
            '.bmp': 'image/bmp',
        }
        
        return mime_types.get(extension, 'image/jpeg')

    def _generate_description(self, image_bytes: bytes, mime_type: str) -> str:
        """Generate a text description of the image using vision model.
        
        Args:
            image_bytes: Raw image content.
            mime_type: MIME type of the image.
            
        Returns:
            Text description of the image.
        """
        try:
            # Encode image to base64
            base64_image = base64.b64encode(image_bytes).decode('utf-8')
            
            # Call OpenAI vision model
            response = self.openai_client.chat.completions.create(
                model="gpt-4o-mini",  # Vision-capable model
                messages=[
                    {
                        "role": "user",
                        "content": [
                            {
                                "type": "text",
                                "text": (
                                    "Please describe this image in detail for a knowledge base. "
                                    "Include:\n"
                                    "1. Main subject/content of the image\n"
                                    "2. Any text visible in the image\n"
                                    "3. Important visual elements, colors, or patterns\n"
                                    "4. Context or setting if applicable\n"
                                    "Be comprehensive but concise."
                                ),
                            },
                            {
                                "type": "image_url",
                                "image_url": {
                                    "url": f"data:{mime_type};base64,{base64_image}",
                                    "detail": "high",
                                },
                            },
                        ],
                    }
                ],
                max_tokens=1000,
            )
            
            return response.choices[0].message.content or ""
            
        except Exception as e:
            logger.error(f"Failed to generate image description: {e}")
            return ""

    def _extract_ocr_text(self, image_bytes: bytes) -> str | None:
        """Extract text from image using OCR.
        
        This is a placeholder for OCR integration. You can integrate
        with libraries like pytesseract or cloud OCR services.
        
        Args:
            image_bytes: Raw image content.
            
        Returns:
            Extracted text or None.
        """
        # Placeholder for OCR integration
        # To implement, you could use:
        # - pytesseract (requires Tesseract installed)
        # - Google Cloud Vision API
        # - AWS Textract
        # - Azure Computer Vision
        
        try:
            # Try pytesseract if available
            import pytesseract
            from PIL import Image
            
            image = Image.open(io.BytesIO(image_bytes))
            text = pytesseract.image_to_string(image)
            return text.strip() if text.strip() else None
            
        except ImportError:
            logger.debug("pytesseract not available, skipping OCR")
            return None
        except Exception as e:
            logger.warning(f"OCR extraction failed: {e}")
            return None

    def _combine_text(self, description: str, ocr_text: str | None) -> str:
        """Combine description and OCR text into a single string.
        
        Args:
            description: AI-generated description.
            ocr_text: OCR-extracted text.
            
        Returns:
            Combined text for embedding.
        """
        parts = []
        
        if description:
            parts.append(f"Image Description:\n{description}")
        
        if ocr_text:
            parts.append(f"Text in Image:\n{ocr_text}")
        
        return "\n\n".join(parts)

    def _create_chunks(
        self,
        combined_text: str,
        description: str,
        ocr_text: str | None,
    ) -> list[ImageChunk]:
        """Create chunks from the combined text.
        
        For most images, this will be a single chunk since descriptions
        are typically short. Longer content is split appropriately.
        
        Args:
            combined_text: Full text to chunk.
            description: Original description.
            ocr_text: Original OCR text.
            
        Returns:
            List of ImageChunk objects.
        """
        if len(combined_text) <= self.chunk_size:
            return [ImageChunk(
                text=combined_text,
                chunk_index=0,
                image_description=description,
                ocr_text=ocr_text,
            )]
        
        # Split into multiple chunks
        chunks: list[ImageChunk] = []
        start = 0
        
        while start < len(combined_text):
            end = min(start + self.chunk_size, len(combined_text))
            
            # Try to break at paragraph or sentence boundary
            if end < len(combined_text):
                for sep in ['\n\n', '. ', '.\n']:
                    last_sep = combined_text.rfind(sep, start, end)
                    if last_sep > start:
                        end = last_sep + len(sep)
                        break
            
            chunk_text = combined_text[start:end].strip()
            if chunk_text:
                chunks.append(ImageChunk(
                    text=chunk_text,
                    chunk_index=len(chunks),
                    image_description=description,
                    ocr_text=ocr_text,
                ))
            
            # Move to next position with overlap
            start = max(start + 1, end - self.chunk_overlap)
        
        return chunks

    def extract_metadata(self, image_bytes: bytes) -> dict[str, any]:
        """Extract metadata from an image.
        
        Args:
            image_bytes: Raw image content.
            
        Returns:
            Dictionary of extracted metadata.
        """
        metadata: dict[str, any] = {}
        
        try:
            from PIL import Image
            from PIL.ExifTags import TAGS
            
            image = Image.open(io.BytesIO(image_bytes))
            
            metadata['format'] = image.format
            metadata['mode'] = image.mode
            metadata['width'] = image.width
            metadata['height'] = image.height
            
            # Extract EXIF data if available
            exif_data = image._getexif()
            if exif_data:
                for tag_id, value in exif_data.items():
                    tag = TAGS.get(tag_id, tag_id)
                    if isinstance(value, (str, int, float)):
                        metadata[f'exif_{tag}'] = value
                        
        except ImportError:
            logger.debug("PIL not available, skipping image metadata extraction")
        except Exception as e:
            logger.warning(f"Failed to extract image metadata: {e}")
        
        return metadata
