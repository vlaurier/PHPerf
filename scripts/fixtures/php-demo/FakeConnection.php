<?php
/**
 * PHPerf — scénario de démonstration (application PHP minimale, sans framework).
 *
 * Simule des anti-patterns classiques que PHPerf doit repérer :
 *   - N+1 : 50 appels à une fausse connexion Doctrine\DBAL dans une boucle ;
 *   - CPU  : hachages répétés sur des charges artificielles.
 *
 * Aucune dépendance, aucune extension requise pour s'exécuter ; ext-xhprof
 * n'est nécessaire que pour le profilage par phperf-profile.php.
 *
 * Usage :
 *   php scripts/php/phperf-profile.php \
 *       --output=bin/phperf-demo.json scripts/fixtures/php-demo/scenario.php
 */

declare(strict_types=1);

namespace Doctrine\DBAL;

// Porteur du nom attendu par la règle n-plus-one-query :
// callee "^Doctrine\\DBAL..." appelé en boucle.
final class FakeConnection
{
    public function query(string $sql): array
    {
        usleep(2000); // latence réseau simulée

        return ['rows' => [$sql]];
    }
}
