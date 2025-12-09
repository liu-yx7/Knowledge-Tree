# Embed AI Development Prompt

This prompt guides the development of an embedded AI helper feature on the `/memos` page.

## Context
- **Project**: Knowledge Tree (Go backend, React+Vite frontend).
- **Goal**: Add a floating AI chat helper to the `/memos` page (Home).
- **Existing AI Feature**: There is a full-page AI chat at `/ai` (`web/src/pages/AIChat.tsx`) using `web/src/components/ChatInterface.tsx` and `web/src/store/ai.ts`.

## Requirements

1.  **Entrance**:
    -   Add a floating action button (FAB) at the bottom-right corner of the `/memos` page (`web/src/pages/Home.tsx`).
    -   Icon: Use `BotIcon` from `lucide-react`.
    -   Style: Consistent with the application theme (e.g., primary color, rounded).

2.  **Interaction**:
    -   Clicking the button opens a chat interface.
    -   Use the `Sheet` component (`web/src/components/ui/sheet.tsx`) to create a slide-over panel from the right side.
    -   The panel should be dismissible.

3.  **Functionality**:
    -   **Reuse**: MUST reuse the existing `ChatInterface` component (`web/src/components/ChatInterface.tsx`) and `aiStore` (`web/src/store/ai.ts`).
    -   **Data Sync**: The chat history and context should be consistent with the `/ai` page.
    -   **Conversation Management**: 
        -   The embedded view should allow creating a new conversation or continuing the last active one.
        -   Ideally, show a simplified list of recent conversations or a "New Chat" button within the header of the embedded sheet.

## Step-by-Step Implementation Plan

### Phase 1: Create Embedded Chat Component

Create a new component `web/src/components/EmbedAIChat.tsx`.

**Specifications:**
-   Import `Sheet`, `SheetContent`, `SheetTrigger` from `@/components/ui/sheet`.
-   Import `ChatInterface` from `@/components/ChatInterface`.
-   **Layout**:
    -   The `SheetContent` should contain the `ChatInterface`.
    -   Adjust styling to ensure `ChatInterface` takes up the full height of the sheet content.
-   **State**:
    -   Use `aiStore` to manage the conversation state.
    -   When the sheet opens, ensure `aiStore.fetchConversations()` is called if not already loaded.
    -   Handle `onConversationCreated` prop of `ChatInterface` to update the local view (e.g., maybe just refresh the store or set the current conversation).

### Phase 2: Integrate into Home Page

Modify `web/src/pages/Home.tsx`:
-   Import `EmbedAIChat`.
-   Place the `EmbedAIChat` component at the root of the `Home` component's return JSX (or inside the main wrapper).
-   The `EmbedAIChat` will handle its own trigger button rendering (via `SheetTrigger`), or you can separate the trigger if needed.
    -   *Recommendation*: Let `EmbedAIChat` render the FAB as its `SheetTrigger`.
-   **Positioning**:
    -   Use absolute or fixed positioning for the trigger button: `fixed bottom-6 right-6 z-50`.

### Phase 3: Refinement

-   **ChatInterface Adaptation**:
    -   Review `web/src/components/ChatInterface.tsx`. It currently assumes it's in a flex container (`flex-1`). Ensure it behaves correctly inside the `Sheet`.
    -   The `ChatInterface` has a header `aiStore.currentConversation.name`. Ensure this looks good in the Sheet, or hide it if the Sheet header covers it.
-   **Store Handling**:
    -   Ensure that closing the sheet doesn't destroy the chat state unexpectedly, but also consider if we want to reset `aiStore.currentConversation` when leaving the context. For now, keeping it active is fine.

## Reference Files

-   `web/src/pages/Home.tsx`: Target page.
-   `web/src/pages/AIChat.tsx`: Reference implementation.
-   `web/src/components/ChatInterface.tsx`: Core component.
-   `web/src/components/ui/sheet.tsx`: UI container.

## Task

Generate the code changes to implement the Embed AI feature following the plan above.
