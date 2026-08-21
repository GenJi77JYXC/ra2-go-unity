using UnityEngine;

// Delegates entirely to the scene's Grid component instead of
// reimplementing Unity's isometric cell<->world math by hand. A hand-rolled
// formula has to guess exactly how Unity positions a tile within its cell
// (corner vs. center, and any pivot offset) — get that guess wrong and
// units render visibly misaligned from the Tilemap they're supposedly
// standing on. Going through the same Grid the Tilemap itself uses
// guarantees units and click targets always agree with wherever tiles
// actually render, no guessing involved.
public static class IsoCoordConverter
{
    public static Vector3 CellToWorld(Grid grid, double cellX, double cellY)
    {
        return grid.CellToLocalInterpolated(new Vector3((float)cellX, (float)cellY, 0f));
    }

    public static Vector2 WorldToCell(Grid grid, Vector3 worldPosition)
    {
        Vector3 cell = grid.LocalToCellInterpolated(worldPosition);
        return new Vector2(cell.x, cell.y);
    }
}
