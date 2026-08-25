<?php
/**
 * PHPerf — collecte XHProf via Composer autoload.
 *
 * Ce fichier est chargé automatiquement dès vendor/autoload.php. Il est
 * entièrement transparent sauf si PHPERF_PROFILE=1 est posé dans
 * l'environnement PHP (requête HTTP, CLI, conteneur, etc.).
 *
 * Comportement :
 *   - Sans PHPERF_PROFILE : retour immédiat, zéro coût.
 *   - Avec PHPERF_PROFILE=1 : active XHProf, écrit un profil JSON
 *     horodaté en fin de script dans PHPERF_OUTPUT_DIR (défaut : sys_get_temp_dir()).
 *
 * Variables reconnues :
 *   PHPERF_PROFILE=1      active la collecte
 *   PHPERF_OUTPUT_DIR=...  répertoire cible (défaut : tmp système)
 *   PHPERF_FLAGS=...       flags XHProf séparés par des virgules (défaut : cpu,memory)
 */
declare(strict_types=1);

(function (): void {
    if ((getenv('PHPERF_PROFILE') ?: '') !== '1') {
        return;
    }

    if (!function_exists('xhprof_enable')) {
        error_log('[phperf] PHPERF_PROFILE=1 mais ext-xhprof absente — collecte ignorée');
        return;
    }

    $flags = 0;
    $raw   = getenv('PHPERF_FLAGS') ?: 'cpu,memory';
    foreach (explode(',', $raw) as $name) {
        $name = trim($name);
        if (!defined("XHPROF_FLAGS_{$name}")) {
            continue;
        }
        $flags |= constant("XHPROF_FLAGS_{$name}");
    }
    if ($flags === 0) {
        $flags = XHPROF_FLAGS_CPU | XHPROF_FLAGS_MEMORY;
    }

    xhprof_enable($flags);

    register_shutdown_function(static function (): void {
        if (!function_exists('xhprof_disable')) {
            return;
        }
        $data = xhprof_disable();

        // Normalisation : entiers garantis, champs mémoire présents,
        // racine "main()" toujours définie.
        $maxWt = 0;
        foreach ($data as $key => $row) {
            $data[$key] = [
                'ct' => (int) ($row['ct'] ?? 0),
                'wt' => (int) ($row['wt'] ?? 0),
                'cpu' => (int) ($row['cpu'] ?? 0),
                'mu' => (int) ($row['mu'] ?? 0),
                'pmu' => (int) ($row['pmu'] ?? 0),
            ];
            if ($key !== 'main()') {
                $maxWt = max($maxWt, $data[$key]['wt']);
            }
        }
        if (!isset($data['main()'])) {
            $data = ['main()' => ['ct' => 1, 'wt' => $maxWt, 'cpu' => $maxWt,
                                  'mu' => 0, 'pmu' => 0]] + $data;
        }

        $json = json_encode(
            $data,
            JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE
        );
        if ($json === false) {
            error_log('[phperf] encodage JSON impossible : ' . json_last_error_msg());
            return;
        }

        $dir  = rtrim(getenv('PHPERF_OUTPUT_DIR') ?: sys_get_temp_dir(), '/');
        $file = $dir . '/phperf-' . date('Ymd-His') . '-' . bin2hex(random_bytes(3)) . '.json';

        if (@file_put_contents($file, $json . "\n") === false) {
            error_log("[phperf] écriture du profil impossible dans $file");
            return;
        }
        error_log("[phperf] profil écrit : $file (" . count($data) . " entrées)");
    });
})();
