import '../l10n/gen/app_localizations.dart';
import '../models.dart';
import '../settings.dart';

/// Celé minuty (dolů) ze sekund.
int minutesOf(int seconds) => seconds <= 0 ? 0 : seconds ~/ 60;

/// "za 6 min" / "za chvíli".
String etaText(AppLocalizations l, int etaMin) => etaMin <= 0 ? l.etaSoon : l.etaInMin(etaMin);

/// Krátká délka: "3 min" / "< 1 min".
String durationText(AppLocalizations l, int seconds) {
  final m = minutesOf(seconds);
  return m < 1 ? l.lessThanMinute : l.minutesShort(m);
}

String itemStatusLabel(AppLocalizations l, ItemStatus s) => switch (s) {
  ItemStatus.hold => l.statusHold,
  ItemStatus.newItem => l.statusNew,
  ItemStatus.claimed => l.statusClaimed,
  ItemStatus.cooking => l.statusCooking,
  ItemStatus.ready => l.statusReady,
  ItemStatus.unknown => '',
};

String tableStatusLabel(AppLocalizations l, TableStatus s) => switch (s) {
  TableStatus.waiting => l.tableWaiting,
  TableStatus.cooking => l.statusCooking,
  TableStatus.partial => l.tablePartial,
  TableStatus.ready => l.tableReady,
  TableStatus.hold => l.statusHold,
  TableStatus.unknown => '',
};

String displayModeLabel(AppLocalizations l, DisplayMode m) => switch (m) {
  DisplayMode.kitchen => l.modeKitchen,
  DisplayMode.pass => l.modePass,
  DisplayMode.floor => l.modeFloor,
};

String qtyName(int qty, String name) => qty > 1 ? '$qty× $name' : name;
