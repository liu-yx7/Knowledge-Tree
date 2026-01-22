"""Document indexer for RAG."""

import logging
from datetime import datetime

from openai import OpenAI
from sqlalchemy.orm import Session

from src.config import get_settings
from src.db.models import DocumentEmbedding
from src.rag.vector_store import delete_document_embeddings

logger = logging.getLogger(__name__)


def chunk_text(text: str, chunk_size: int = 1000, overlap: int = 200) -> list[str]:
    """Split text into overlapping chunks.
    
    This is a standalone utility function for chunking text.
    
    Args:
        text: Text to chunk
        chunk_size: Maximum size of each chunk (default: 1000)
        overlap: Number of characters to overlap between chunks (default: 200)
        
    Returns:
        List of text chunks
    """
    if not text:
        return []
        
    if len(text) <= chunk_size:
        return [text]

    chunks = []
    start = 0
    while start < len(text):
        end = start + chunk_size
        chunk = text[start:end]
        chunks.append(chunk)
        start = end - overlap
        
        # Prevent infinite loop if overlap >= chunk_size
        if start <= 0 and end >= len(text):
            break

    return chunks


class DocumentIndexer:
    """Indexes documents for RAG by generating embeddings."""

    def __init__(self, db: Session):
        self.db = db
        self.settings = get_settings()
        self.openai_client = OpenAI(api_key=self.settings.openai_api_key)

    def _chunk_text(self, text: str) -> list[str]:
        """Split text into overlapping chunks.
        
        Args:
            text: Text to chunk
            
        Returns:
            List of text chunks
        """
        chunk_size = self.settings.rag_chunk_size
        overlap = self.settings.rag_chunk_overlap

        if len(text) <= chunk_size:
            return [text]

        chunks = []
        start = 0
        while start < len(text):
            end = start + chunk_size
            chunk = text[start:end]
            chunks.append(chunk)
            start = end - overlap

        return chunks

    def _generate_embeddings(self, texts: list[str]) -> list[list[float]]:
        """Generate embeddings for a list of texts.
        
        Args:
            texts: List of text strings
            
        Returns:
            List of embedding vectors
        """
        response = self.openai_client.embeddings.create(
            model=self.settings.embedding_model,
            input=texts,
            dimensions=self.settings.embedding_dimensions,
        )
        return [item.embedding for item in response.data]

    def index_memo(
        self,
        user_id: int,
        memo_uid: str,
        content: str,
    ) -> int:
        """Index a memo by generating embeddings for its content.
        
        Args:
            user_id: User ID who owns the memo
            memo_uid: UID of the memo
            content: Memo content
            
        Returns:
            Number of chunks indexed
        """
        # Delete existing embeddings for this memo
        delete_document_embeddings(self.db, "memo", memo_uid)

        if not content.strip():
            return 0

        # Chunk the content
        chunks = self._chunk_text(content)

        # Generate embeddings
        embeddings = self._generate_embeddings(chunks)

        # Store embeddings
        now = int(datetime.utcnow().timestamp())
        for i, (chunk, embedding) in enumerate(zip(chunks, embeddings)):
            doc_embedding = DocumentEmbedding(
                user_id=user_id,
                document_type="memo",
                document_uid=memo_uid,
                chunk_index=i,
                chunk_text=chunk,
                embedding=embedding,
                created_ts=now,
                updated_ts=now,
            )
            self.db.add(doc_embedding)

        self.db.commit()
        logger.info(f"Indexed memo {memo_uid}: {len(chunks)} chunks")
        return len(chunks)

    def delete_memo_index(self, memo_uid: str) -> int:
        """Delete all embeddings for a memo.
        
        Args:
            memo_uid: UID of the memo
            
        Returns:
            Number of deleted chunks
        """
        count = delete_document_embeddings(self.db, "memo", memo_uid)
        logger.info(f"Deleted index for memo {memo_uid}: {count} chunks")
        return count

    def index_attachment(
        self,
        user_id: int,
        attachment_uid: str,
        content: str,
    ) -> int:
        """Index an attachment by generating embeddings for its content.
        
        Args:
            user_id: User ID who owns the attachment
            attachment_uid: UID of the attachment
            content: Extracted text content from the attachment
            
        Returns:
            Number of chunks indexed
        """
        # Delete existing embeddings for this attachment
        delete_document_embeddings(self.db, "attachment", attachment_uid)

        if not content.strip():
            return 0

        # Chunk the content
        chunks = self._chunk_text(content)

        # Generate embeddings
        embeddings = self._generate_embeddings(chunks)

        # Store embeddings
        now = int(datetime.utcnow().timestamp())
        for i, (chunk, embedding) in enumerate(zip(chunks, embeddings)):
            doc_embedding = DocumentEmbedding(
                user_id=user_id,
                document_type="attachment",
                document_uid=attachment_uid,
                chunk_index=i,
                chunk_text=chunk,
                embedding=embedding,
                created_ts=now,
                updated_ts=now,
            )
            self.db.add(doc_embedding)

        self.db.commit()
        logger.info(f"Indexed attachment {attachment_uid}: {len(chunks)} chunks")
        return len(chunks)
