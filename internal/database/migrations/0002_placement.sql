-- Where an item is shown: the header toolbar menu, a card on the home tab, or both.
ALTER TABLE menu_items
    ADD COLUMN placement TEXT NOT NULL DEFAULT 'both' CHECK (placement IN ('menu', 'card', 'both'));
