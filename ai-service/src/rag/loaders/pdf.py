"""PDF document loader for RAG indexing.

This module provides functionality to extract text from PDF attachments
and chunk them for vector embedding and retrieval.
"""

import io
import logging
from dataclasses import dataclass
from pathlib import Path

from src.config import get_settings

logger = logging.getLogger(__name__)


@dataclass
class PDFChunk:
    """A chunk of PDF content for embedding."""
    
    text: str
    chunk_index: int
    page_numbers: list[int]  # Pages this chunk spans


class PDFLoader:
    """Loader for PDF documents that handles text extraction and chunking.
    
    Uses pypdf for text extraction. For scanned PDFs or images,
    consider using OCR (not implemented here).
    """

    def __init__(
        self,
        chunk_size: int | None = None,
        chunk_overlap: int | None = None,
    ):
        """Initialize the PDF loader.
        
        Args:
            chunk_size: Maximum characters per chunk. Defaults to config value.
            chunk_overlap: Overlap between chunks. Defaults to config value.
        """
        settings = get_settings()
        self.chunk_size = chunk_size or settings.rag_chunk_size
        self.chunk_overlap = chunk_overlap or settings.rag_chunk_overlap

    def load_from_path(self, file_path: str | Path) -> list[PDFChunk]:
        """Load and chunk a PDF from a file path.
        
        Args:
            file_path: Path to the PDF file.
            
        Returns:
            List of PDFChunk objects ready for embedding.
        """
        try:
            from pypdf import PdfReader
        except ImportError:
            logger.error("pypdf not installed. Run: pip install pypdf")
            return []

        try:
            reader = PdfReader(file_path)
            return self._process_pdf(reader)
        except Exception as e:
            logger.error(f"Failed to load PDF from {file_path}: {e}")
            return []

    def load_from_bytes(self, pdf_bytes: bytes) -> list[PDFChunk]:
        """Load and chunk a PDF from bytes.
        
        Args:
            pdf_bytes: Raw PDF file content.
            
        Returns:
            List of PDFChunk objects ready for embedding.
        """
        try:
            from pypdf import PdfReader
        except ImportError:
            logger.error("pypdf not installed. Run: pip install pypdf")
            return []

        try:
            reader = PdfReader(io.BytesIO(pdf_bytes))
            return self._process_pdf(reader)
        except Exception as e:
            logger.error(f"Failed to load PDF from bytes: {e}")
            return []

    def _process_pdf(self, reader) -> list[PDFChunk]:
        """Process a PDF reader and extract chunked text.
        
        Args:
            reader: PdfReader instance.
            
        Returns:
            List of PDFChunk objects.
        """
        # Extract text from all pages with page numbers
        pages_text: list[tuple[int, str]] = []
        
        for page_num, page in enumerate(reader.pages):
            try:
                text = page.extract_text()
                if text and text.strip():
                    pages_text.append((page_num + 1, self._clean_text(text)))
            except Exception as e:
                logger.warning(f"Failed to extract text from page {page_num + 1}: {e}")
                continue

        if not pages_text:
            logger.warning("No text extracted from PDF")
            return []

        # Combine all text for chunking
        return self._chunk_pages(pages_text)

    def _clean_text(self, text: str) -> str:
        """Clean extracted PDF text.
        
        Args:
            text: Raw extracted text.
            
        Returns:
            Cleaned text.
        """
        import re
        
        # Normalize whitespace
        text = re.sub(r'[ \t]+', ' ', text)
        
        # Fix common PDF extraction issues
        # - Hyphenated words at line breaks
        text = re.sub(r'-\n(\w)', r'\1', text)
        
        # Normalize line endings
        text = text.replace('\r\n', '\n').replace('\r', '\n')
        
        # Remove excessive newlines but keep paragraph breaks
        text = re.sub(r'\n{3,}', '\n\n', text)
        
        return text.strip()

    def _chunk_pages(self, pages_text: list[tuple[int, str]]) -> list[PDFChunk]:
        """Chunk text from multiple pages.
        
        Maintains page number tracking for each chunk.
        
        Args:
            pages_text: List of (page_number, text) tuples.
            
        Returns:
            List of PDFChunk objects.
        """
        chunks: list[PDFChunk] = []
        current_text = ""
        current_pages: list[int] = []
        
        for page_num, text in pages_text:
            # Try to add this page's text to current chunk
            test_text = current_text + ('\n\n' if current_text else '') + text
            
            if len(test_text) <= self.chunk_size:
                current_text = test_text
                if page_num not in current_pages:
                    current_pages.append(page_num)
            else:
                # Save current chunk if not empty
                if current_text:
                    chunks.append(PDFChunk(
                        text=current_text,
                        chunk_index=len(chunks),
                        page_numbers=current_pages.copy(),
                    ))
                
                # Start new chunk with overlap
                if len(current_text) > self.chunk_overlap:
                    overlap = current_text[-self.chunk_overlap:]
                    current_text = overlap + '\n\n' + text
                    # Keep last page for overlap context
                    current_pages = current_pages[-1:] + [page_num] if current_pages else [page_num]
                else:
                    current_text = text
                    current_pages = [page_num]
                
                # If single page text exceeds chunk size, split it
                if len(current_text) > self.chunk_size:
                    sub_chunks = self._split_large_text(current_text, page_num)
                    chunks.extend(sub_chunks[:-1])  # Add all but last
                    if sub_chunks:
                        current_text = sub_chunks[-1].text
                        current_pages = sub_chunks[-1].page_numbers

        # Don't forget the last chunk
        if current_text:
            chunks.append(PDFChunk(
                text=current_text,
                chunk_index=len(chunks),
                page_numbers=current_pages,
            ))

        # Re-index chunks
        for i, chunk in enumerate(chunks):
            chunk.chunk_index = i

        return chunks

    def _split_large_text(self, text: str, page_num: int) -> list[PDFChunk]:
        """Split a large text block into chunks.
        
        Args:
            text: Text to split.
            page_num: Page number for this text.
            
        Returns:
            List of PDFChunk objects.
        """
        chunks: list[PDFChunk] = []
        start = 0
        
        while start < len(text):
            end = start + self.chunk_size
            
            # Try to break at a sentence or word boundary
            if end < len(text):
                # Look for sentence boundary
                for sep in ['. ', '.\n', '? ', '?\n', '! ', '!\n']:
                    last_sep = text.rfind(sep, start, end)
                    if last_sep > start:
                        end = last_sep + 1
                        break
                else:
                    # Fall back to word boundary
                    last_space = text.rfind(' ', start, end)
                    if last_space > start:
                        end = last_space
            
            chunk_text = text[start:end].strip()
            if chunk_text:
                chunks.append(PDFChunk(
                    text=chunk_text,
                    chunk_index=len(chunks),
                    page_numbers=[page_num],
                ))
            
            # Move start with overlap
            start = end - self.chunk_overlap if end - self.chunk_overlap > start else end

        return chunks

    def extract_metadata(self, reader) -> dict[str, any]:
        """Extract metadata from a PDF.
        
        Args:
            reader: PdfReader instance.
            
        Returns:
            Dictionary of extracted metadata.
        """
        metadata: dict[str, any] = {
            'page_count': len(reader.pages),
        }
        
        if reader.metadata:
            if reader.metadata.title:
                metadata['title'] = reader.metadata.title
            if reader.metadata.author:
                metadata['author'] = reader.metadata.author
            if reader.metadata.subject:
                metadata['subject'] = reader.metadata.subject
            if reader.metadata.creation_date:
                metadata['creation_date'] = str(reader.metadata.creation_date)
        
        return metadata
