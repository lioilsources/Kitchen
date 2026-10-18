import 'package:flutter/material.dart';

import '../api.dart';
import '../models.dart';
import '../widgets/breathing.dart';
import '../widgets/format.dart';
import '../widgets/palette.dart';

/// Tabule na plac: jen čtení, čitelné z 3–5 m.
class FloorScreen extends StatelessWidget {
  const FloorScreen({super.key, required this.client});

  final PredavkaClient client;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: client,
      builder: (context, _) {
        final s = client.snapshot;
        return LayoutBuilder(
          builder: (context, c) {
            final scale = (c.maxWidth / 1280).clamp(0.55, 1.6);
            if (s == null) {
              return _Message('Připojuji se k serveru…', scale: scale);
            }
            if (s.tables.isEmpty) {
              return _Message('Nic se nevaří.', scale: scale);
            }
            return ListView.separated(
              padding: EdgeInsets.fromLTRB(20 * scale, 8, 20 * scale, 140 * scale),
              itemCount: s.tables.length,
              separatorBuilder: (_, _) => SizedBox(height: 12 * scale),
              itemBuilder: (context, i) => FloorRow(key: ValueKey(s.tables[i].table), table: s.tables[i], scale: scale),
            );
          },
        );
      },
    );
  }
}

class FloorRow extends StatelessWidget {
  const FloorRow({super.key, required this.table, this.scale = 1});

  final TableView table;
  final double scale;

  @override
  Widget build(BuildContext context) {
    final t = table;
    final ready = t.status == TableStatus.ready;
    final hold = t.status == TableStatus.hold;
    final alert = Palette.alertColor(t.alert);
    final base = ready ? Color.alphaBlend(Palette.ready.withValues(alpha: 0.20), Palette.surface) : Palette.surface;

    final statusColor = switch (t.status) {
      TableStatus.ready => Palette.ready,
      TableStatus.partial => Palette.text,
      TableStatus.hold || TableStatus.waiting => Palette.textDim,
      _ => Palette.text,
    };

    final String eta;
    if (ready) {
      eta = t.readyWaitS >= 60 ? '${minutesOf(t.readyWaitS)} min' : 'teď';
    } else if (hold) {
      eta = '';
    } else {
      eta = etaText(t.etaMin);
    }

    final items = t.items.map((i) => qtyName(i.qty, i.name)).join(', ');
    final progress = t.itemsTotal > 0 && t.status == TableStatus.partial ? ' ${t.itemsReady}/${t.itemsTotal}' : '';

    return Opacity(
      opacity: hold ? 0.6 : 1,
      child: Breathing(
        color: alert,
        baseColor: base,
        borderRadius: BorderRadius.circular(18 * scale),
        child: Padding(
          padding: EdgeInsets.symmetric(horizontal: 28 * scale, vertical: 16 * scale),
          child: Row(
            children: [
              SizedBox(
                width: 150 * scale,
                child: FittedBox(
                  fit: BoxFit.scaleDown,
                  alignment: Alignment.centerLeft,
                  child: Text(
                    t.table,
                    maxLines: 1,
                    style: TextStyle(fontSize: 84 * scale, height: 1, fontWeight: FontWeight.w800, color: Palette.text),
                  ),
                ),
              ),
              SizedBox(width: 20 * scale),
              Expanded(
                flex: 5,
                child: Text(
                  items,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 32 * scale, height: 1.2, color: Palette.textDim),
                ),
              ),
              SizedBox(width: 20 * scale),
              Expanded(
                flex: 3,
                child: FittedBox(
                  fit: BoxFit.scaleDown,
                  alignment: Alignment.centerRight,
                  child: Text(
                    eta,
                    maxLines: 1,
                    style: TextStyle(
                      fontSize: 52 * scale,
                      fontWeight: FontWeight.w700,
                      color: ready ? (alert ?? Palette.ready) : Palette.text,
                    ),
                  ),
                ),
              ),
              SizedBox(width: 24 * scale),
              Expanded(
                flex: 4,
                child: Text(
                  tableStatusLabel(t.status) + progress,
                  textAlign: TextAlign.right,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: (ready ? 36 : 32) * scale,
                    fontWeight: ready ? FontWeight.w800 : FontWeight.w500,
                    color: statusColor,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Message extends StatelessWidget {
  const _Message(this.text, {required this.scale});
  final String text;
  final double scale;

  @override
  Widget build(BuildContext context) => Center(
    child: Text(
      text,
      style: TextStyle(fontSize: 44 * scale, color: Palette.textFaint),
    ),
  );
}
