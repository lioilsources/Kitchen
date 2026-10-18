import 'package:flutter/material.dart';

import '../models.dart';
import 'breathing.dart';
import 'format.dart';
import 'palette.dart';

/// Velký kuchyňský lístek – čitelný na dálku.
class TicketCard extends StatelessWidget {
  const TicketCard({
    super.key,
    required this.item,
    required this.onTap,
    this.index,
    this.selected = false,
    this.flash = false,
  });

  final ItemView item;
  final VoidCallback onTap;

  /// Číslo pro bump bar (1–9), null = bez čísla.
  final int? index;
  final bool selected;

  /// Krátká potvrzovací odezva po bumpu.
  final bool flash;

  @override
  Widget build(BuildContext context) {
    final hold = item.status == ItemStatus.hold;
    final isNew = item.status == ItemStatus.newItem;
    final alertColor = hold ? null : Palette.alertColor(item.alert);
    final base = isNew ? Color.alphaBlend(Palette.newItem.withValues(alpha: 0.06), Palette.surface) : Palette.surface;

    final waitLabel = switch (item.status) {
      ItemStatus.newItem => 'čeká ${durationText(item.waitS)}',
      ItemStatus.claimed || ItemStatus.cooking => 'vaří ${durationText(item.waitS)}',
      ItemStatus.hold => 'chod ${item.course}',
      _ => durationText(item.waitS),
    };

    final borderColor = flash
        ? Palette.ok
        : selected
        ? Palette.text
        : isNew
        ? Palette.newItem.withValues(alpha: 0.45)
        : Palette.outline;

    final card = Breathing(
      color: alertColor,
      baseColor: base,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 350),
        curve: Curves.easeOut,
        padding: const EdgeInsets.fromLTRB(20, 14, 20, 16),
        decoration: BoxDecoration(
          color: flash ? Palette.ok.withValues(alpha: 0.18) : Colors.transparent,
          borderRadius: BorderRadius.circular(16),
          border: Border.all(color: borderColor, width: selected || flash ? 3 : 1.5),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (index != null)
                  Container(
                    margin: const EdgeInsets.only(right: 12, top: 10),
                    width: 34,
                    height: 34,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: selected ? Palette.text : Palette.surfaceHigh,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Text(
                      '$index',
                      style: TextStyle(
                        fontSize: 20,
                        fontWeight: FontWeight.w700,
                        color: selected ? Palette.background : Palette.textDim,
                      ),
                    ),
                  ),
                Expanded(
                  child: FittedBox(
                    fit: BoxFit.scaleDown,
                    alignment: Alignment.centerLeft,
                    child: Text(
                      item.table,
                      style: const TextStyle(
                        fontSize: 64,
                        height: 1.0,
                        fontWeight: FontWeight.w800,
                        color: Palette.text,
                      ),
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Flexible(
                  flex: 2,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Text(
                        waitLabel,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 22,
                          fontWeight: FontWeight.w600,
                          color: alertColor ?? Palette.textDim,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        _rightSub(),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontSize: 18, color: Palette.textDim),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 10),
            Text(
              qtyName(item.qty, item.name),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 32, fontWeight: FontWeight.w600, color: Palette.text, height: 1.15),
            ),
            if (item.mods.isNotEmpty) ...[
              const SizedBox(height: 6),
              Text(item.mods.join(' · '), style: const TextStyle(fontSize: 22, color: Palette.warn, height: 1.2)),
            ],
            if (item.note.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(
                item.note,
                style: const TextStyle(fontSize: 20, fontStyle: FontStyle.italic, color: Palette.textDim),
              ),
            ],
            const SizedBox(height: 10),
            Row(
              children: [
                Text(
                  itemStatusLabel(item.status),
                  style: TextStyle(
                    fontSize: 18,
                    color: isNew ? Palette.newItem : Palette.textDim,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                if (item.claimedBy.isNotEmpty) ...[
                  const SizedBox(width: 8),
                  Flexible(
                    child: Text(
                      '· ${item.claimedBy}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 18, color: Palette.textDim),
                    ),
                  ),
                ],
                const Spacer(),
                if (flash) const Icon(Icons.check_rounded, color: Palette.ok, size: 28),
              ],
            ),
          ],
        ),
      ),
    );

    return Opacity(
      opacity: hold ? 0.45 : 1,
      child: Material(
        color: Colors.transparent,
        child: InkWell(borderRadius: BorderRadius.circular(16), onTap: onTap, child: card),
      ),
    );
  }

  String _rightSub() {
    if (item.status == ItemStatus.hold) return 'drženo';
    if (item.status == ItemStatus.newItem && item.startInMin > 0) {
      return 'začít za ${item.startInMin} min';
    }
    return 'hotovo ${etaText(item.etaMin)}';
  }
}

/// Malý šedý lístek pro READY položky (konec fronty).
class ReadyChip extends StatelessWidget {
  const ReadyChip({super.key, required this.item, required this.onTap});

  final ItemView item;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final alert = Palette.alertColor(item.alert);
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: onTap,
        child: Breathing(
          color: alert,
          baseColor: Palette.surface.withValues(alpha: 0.6),
          borderRadius: BorderRadius.circular(12),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  item.table,
                  style: const TextStyle(fontSize: 24, fontWeight: FontWeight.w700, color: Palette.textDim),
                ),
                const SizedBox(width: 10),
                Flexible(
                  child: Text(
                    qtyName(item.qty, item.name),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 18, color: Palette.textFaint),
                  ),
                ),
                const SizedBox(width: 10),
                Text(
                  'na předávce ${durationText(item.waitS)}',
                  style: TextStyle(fontSize: 16, color: alert ?? Palette.textFaint),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
