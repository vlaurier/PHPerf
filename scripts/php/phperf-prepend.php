<?php
/**
 * PHPerf — amorce de profilage HTTP (auto_prepend_file).
 *
 * À déclarer une seule fois dans la config PHP du serveur :
 *
 *   auto_prepend_file = /chemin/vers/phperf-prepend.php
 *
 * Le profilage ne s'active que si la variable d'environnement
 * PHPERF_PROFILE=1 est posée — les requêtes ordinaires ne paient donc
 * strictement rien. Ce fichier inclut lui-même phperf-append.php, qui
 * enregistre la fonction d'écriture du profil en fin de requête :
 * auto_append_file est inutile.
 *
 * Variables reconnues :
 *   PHPERF_PROFILE=1        active la collecte pour cette requête ;
 *   PHPERF_OUTPUT_DIR=...   répertoire des dumps (défaut : tmp système).
 */

declare(strict_types=1);

if ((getenv('PHPERF_PROFILE') ?: '') !== '1') {
    return;
}

if (!function_exists('xhprof_enable')) {
    error_log('[phperf] PHPERF_PROFILE=1 mais ext-xhprof absente — profilage ignoré');
    return;
}

$GLOBALS['PHPPERF_ACTIVE'] = true;
xhprof_enable(XHPROF_FLAGS_CPU | XHPROF_FLAGS_MEMORY);

require __DIR__ . '/phperf-append.php';
