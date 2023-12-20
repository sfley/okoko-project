<html>

<head>
<title>conso</title>
</head>

<body>


<script src="<?php echo base_url();?>assets/js/highcharts.js"></script>
<script src="<?php echo base_url();?>assets/js/series-label.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/exporting.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/export-data.js"></script>




<figure class="highcharts-figure">
    <div id="conso_container"></div>
     <div id="conso_day_container"></div>
</figure>

<script type="text/javascript">
Highcharts.chart('conso_container', {
    chart: {
        type: 'column'
    },
    title: {
        text: '<?php echo $conso_annuelle['name']; ?>',
        align: 'center'
    },
    xAxis: {
        categories: [<?php echo $conso_annuelle['mois']; ?>]
    },
    yAxis: {
        min: 0,
        title: {
            text: 'Masse en Kg'
        },
        stackLabels: {
            enabled: true
        }
    },
    tooltip: {
        headerFormat: '<b>{point.x}</b><br/>',
        pointFormat: ' {point.y} Kg<br/>'
    },
    plotOptions: {
        column: {
            
            dataLabels: {
                enabled: true,format: '{y} Kg'
            }
        }
    },
    series: [{
        name: <?php echo $conso_annuelle['annee']; ?>,
        data: [<?php echo $conso_annuelle['conso']; ?>]
    }],
});















</script>

</body>
</html>